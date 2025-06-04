package limiter

import (
	"sync"
	"time"
)

// Every converts a minimum time interval between events to a Limit.
func Every(interval time.Duration) Limit {
	if interval <= 0 {
		return Inf
	}
	return 1 / Limit(interval.Seconds())
}

type Limiter struct {
	mu     sync.Mutex
	limit  Limit
	burst  int
	tokens float64
	// last : 限制器的tokens字段最后一次的更新时间
	last time.Time
	// lastEvent : 速率限制事件（过去或者将来）的最新时间
	lastEvent time.Time
}

// Limit 返回最大总体时间发生率
func (lim *Limiter) Limit() Limit {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	return lim.limit
}

// Burst 返回实际单次触发（限制器）时，所能允许的最大数。
// Burst 是单次调用 Allow、Reserve 或 Wait 时可以消耗的最大令牌数，因此，Burst 值越高，允许同时发生更多事件。
// 如果 Burst 为 0，则不允许发生任何事件，除非 limit == Inf
func (lim *Limiter) Burst() int {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	return lim.burst
}

// TokensAt 返回时间 t 时可用的令牌数。
func (lim *Limiter) TokensAt(t time.Time) float64 {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	tokens := lim.advance(t)
	return tokens
}

// Tokens 返回现在可用的令牌数量。
func (lim *Limiter) Tokens() float64 {
	return lim.TokensAt(time.Now())
}

// NewLimiter 返回一个新的限制器，允许事件速率达到 r，并允许
// 最多 b 个令牌的突发。
// 初始化限制器函数
func NewLimiter(r Limit, b int) *Limiter {
	return &Limiter{
		limit:  r,
		burst:  b,
		tokens: float64(b), // 一开始默认就充满令牌池
	}
}

// advance 计算并返回 lim 的更新令牌数量
// 随着时间的推移。
// lim 保持不变。
// advance 要求保留 lim.mu。-- 该函数功能仅为计算令牌数量，被其他持有锁的函数调用，所以自己不加锁
func (lim *Limiter) advance(t time.Time) (newTokens float64) {
	last := lim.last
	if t.Before(last) {
		// t在last之前
		last = t
	}

	// 随着时间的流逝，计算新的令牌数量。
	// 获取传入时间距离上一次更新时间的秒数
	elapsed := t.Sub(last)
	delta := lim.limit.tokensFormDuration(elapsed)
	tokens := lim.tokens + delta
	if burst := float64(lim.burst); tokens > burst {
		tokens = burst
	}
	return tokens
}
