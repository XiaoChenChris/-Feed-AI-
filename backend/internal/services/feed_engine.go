package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	rediscache "feedsystem_ai_go/pkg/cache"
	"feedsystem_ai_go/internal/models"
	"feedsystem_ai_go/internal/repositories"
)

const (
	engineRecallOversample = 3                 // 每路召回的 oversample 倍数
	engineCategoryCap      = 4                 // 同类视频上限，防止同质化疲劳
	engineExposureTTL      = 7 * 24 * time.Hour // 已曝光集合 TTL
	engineInterestTags     = 10                // 兴趣召回取最近点赞的标签数

	// 各召回路径权重（叠加到精排分数上）
	pathWeightLatest    = 0.3
	pathWeightHot       = 0.4
	pathWeightFollowing = 0.8
	pathWeightInterest  = 0.8

	// 精排权重
	wPopularity = 0.4
	wLikes      = 0.3
	wRecency    = 0.2
)

type feedCandidate struct {
	VideoID uint
	Path    string
	Score   float64
}

type scoredVideo struct {
	v *models.Video
	s float64
}

// FeedEngine 统一 feed 引擎：多路并行召回 -> 合并去重 -> 粗筛(曝光/同类上限) -> 精排 -> 分页。
type FeedEngine struct {
	repo     *repositories.FeedRepository
	likeRepo *repositories.LikeRepository
	cache    *rediscache.Client
}

func NewFeedEngine(repo *repositories.FeedRepository, likeRepo *repositories.LikeRepository, cache *rediscache.Client) *FeedEngine {
	return &FeedEngine{repo: repo, likeRepo: likeRepo, cache: cache}
}

func exposureKey(accountID uint) string {
	return fmt.Sprintf("feed:exposed:%d", accountID)
}

// Generate 编排整个 feed 生成流程。
func (e *FeedEngine) Generate(ctx context.Context, accountID uint, limit, cursor int) ([]*models.Video, int, bool, error) {
	recallLimit := limit * engineRecallOversample
	candidates := e.recallParallel(ctx, accountID, recallLimit)

	merged := mergeAndDedup(candidates)
	if len(merged) == 0 {
		return nil, cursor, false, nil
	}

	ids := make([]uint, 0, len(merged))
	for _, c := range merged {
		ids = append(ids, c.VideoID)
	}

	videos, err := e.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, 0, false, err
	}
	if len(videos) == 0 {
		return nil, cursor, false, nil
	}

	catMap, err := e.repo.GetCategoryByIDs(ctx, ids)
	if err != nil {
		return nil, 0, false, err
	}
	byID := make(map[uint]*models.Video, len(videos))
	for _, v := range videos {
		v.Category = catMap[v.ID] // 可能为空（无分类也无 tag 的视频）
		byID[v.ID] = v
	}

	exposed := loadExposed(ctx, accountID, e.cache)

	var fresh, seen []scoredVideo
	for _, c := range merged {
		v, ok := byID[c.VideoID]
		if !ok {
			continue
		}
		item := scoredVideo{v: v, s: c.Score + rankScore(v)}
		if exposed[v.ID] {
			seen = append(seen, item) // 已曝光，先搁置
		} else {
			fresh = append(fresh, item)
		}
	}

	sortByScoreDesc(fresh)
	sortByScoreDesc(seen)

	final := applyCategoryCap(fresh, seen, engineCategoryCap, limit)

	if cursor < 0 {
		cursor = 0
	}
	total := len(final)
	start := cursor
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	page := final[start:end]
	nextCursor := start + len(page)
	hasMore := end < total

	if accountID > 0 && len(page) > 0 {
		members := make([]string, 0, len(page))
		for _, sc := range page {
			members = append(members, strconv.FormatUint(uint64(sc.v.ID), 10))
		}
		e.cache.SAdd(ctx, exposureKey(accountID), members...)
		e.cache.Expire(ctx, exposureKey(accountID), engineExposureTTL)
	}

	out := make([]*models.Video, 0, len(page))
	for _, sc := range page {
		out = append(out, sc.v)
	}
	return out, nextCursor, hasMore, nil
}

// recallParallel 并发执行多路召回，每路返回 oversample 倍的候选视频。
func (e *FeedEngine) recallParallel(ctx context.Context, accountID uint, recallLimit int) []feedCandidate {
	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		collected []feedCandidate
	)
	run := func(path string, weight float64, fn func() ([]*models.Video, error)) {
		defer wg.Done()
		videos, err := fn()
		if err != nil || len(videos) == 0 {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		for _, v := range videos {
			collected = append(collected, feedCandidate{VideoID: v.ID, Path: path, Score: weight})
		}
	}

	wg.Add(2)
	go run("latest", pathWeightLatest, func() ([]*models.Video, error) {
		return e.repo.ListLatest(ctx, recallLimit, time.Time{})
	})
	go run("hot", pathWeightHot, func() ([]*models.Video, error) {
		return e.repo.ListByPopularity(ctx, recallLimit, 0, time.Time{}, 0)
	})
	if accountID > 0 {
		wg.Add(2)
		go run("following", pathWeightFollowing, func() ([]*models.Video, error) {
			return e.repo.ListByFollowing(ctx, recallLimit, accountID, time.Time{})
		})
		go run("interest", pathWeightInterest, func() ([]*models.Video, error) {
			tags, err := e.likeRepo.GetLikedVideoTags(ctx, accountID, engineInterestTags)
			if err != nil || len(tags) == 0 {
				return nil, err
			}
			return e.repo.ListByTags(ctx, tags, recallLimit)
		})
	}
	wg.Wait()
	return collected
}

// mergeAndDedup 按 videoID 合并，重复者保留最高分路径。
func mergeAndDedup(cands []feedCandidate) []feedCandidate {
	best := make(map[uint]feedCandidate, len(cands))
	for _, c := range cands {
		if cur, ok := best[c.VideoID]; !ok || c.Score > cur.Score {
			best[c.VideoID] = c
		}
	}
	out := make([]feedCandidate, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	return out
}

// rankScore 计算单视频的精排基础分（热度 + 点赞 + 时效衰减）。
func rankScore(v *models.Video) float64 {
	pop := math.Log1p(float64(v.Popularity)) * wPopularity
	like := math.Log1p(float64(v.LikesCount)) * wLikes
	hours := time.Since(v.CreateTime).Hours()
	if hours < 0 {
		hours = 0
	}
	recency := (1.0 / (1.0 + hours/24.0)) * wRecency
	return pop + like + recency
}

func sortByScoreDesc(items []scoredVideo) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].s > items[j].s
	})
}

// applyCategoryCap 在 fresh 集合上按同类上限贪心裁剪，不足 limit 时
// 先补被裁掉的同类视频，再补已曝光(seen)视频以避免空流。
func applyCategoryCap(fresh, seen []scoredVideo, capN, limit int) []scoredVideo {
	out := make([]scoredVideo, 0, limit)
	catCount := make(map[string]int)
	var capped []scoredVideo
	for _, sc := range fresh {
		cat := sc.v.Category
		// 空分类（无 category 也无 tag）不参与同类上限，避免新环境整体被截断
		if cat == "" || catCount[cat] < capN {
			out = append(out, sc)
			catCount[cat]++
		} else {
			capped = append(capped, sc)
		}
	}
	// 第一轮裁剪后仍不足，放行被同类上限裁掉的部分
	for _, sc := range capped {
		if len(out) >= limit {
			break
		}
		out = append(out, sc)
	}
	// 仍不足（曝光剔除过狠）则二次放行已曝光视频
	for _, sc := range seen {
		if len(out) >= limit {
			break
		}
		out = append(out, sc)
	}
	return out
}

// loadExposed 读取用户已曝光视频集合（匿名用户返回空）。
func loadExposed(ctx context.Context, accountID uint, cache *rediscache.Client) map[uint]bool {
	out := make(map[uint]bool)
	if accountID == 0 || cache == nil {
		return out
	}
	members, err := cache.SMembers(ctx, exposureKey(accountID))
	if err != nil || len(members) == 0 {
		return out
	}
	for _, m := range members {
		var id uint
		if _, err := fmt.Sscanf(m, "%d", &id); err == nil && id > 0 {
			out[id] = true
		}
	}
	return out
}
