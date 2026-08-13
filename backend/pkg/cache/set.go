package cache

import (
	"context"
)

// SAdd adds members to a set (no-op when cache is disabled).
func (c *Client) SAdd(ctx context.Context, key string, members ...string) error {
	if c == nil || c.rdb == nil || len(members) == 0 {
		return nil
	}
	return c.rdb.SAdd(ctx, key, members).Err()
}

// SRem removes members from a set (no-op when cache is disabled).
func (c *Client) SRem(ctx context.Context, key string, members ...string) error {
	if c == nil || c.rdb == nil || len(members) == 0 {
		return nil
	}
	return c.rdb.SRem(ctx, key, members).Err()
}

// SMembers returns all members of a set. Returns nil when disabled.
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	return c.rdb.SMembers(ctx, key).Result()
}

// SIsMember reports whether member is in the set (false when cache disabled).
func (c *Client) SIsMember(ctx context.Context, key, member string) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	return c.rdb.SIsMember(ctx, key, member).Result()
}
