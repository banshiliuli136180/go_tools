package limiter

import (
	"math"
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

// 预留包含限制器允许在延迟后发生的事件的信息。
// 预留可能会被取消，这可能会使限制器允许其他事件。
// 该结构体表示一次预约令牌的结果（不是目前使用）
type Reservation struct {
	ok        bool // ok 表示是否成功预约
	lim       *Limiter
	tokens    int       // 表示本次预约预约了多少个令牌
	timeToAct time.Time // 预约的何时使用这些令牌
	// 这是预订时的限制，以后可能会改变。
	limit Limit
}

// get私有字段内容
// OK 返回限制器是否能够在最大等待时间内提供请求的令牌数量。
// 如果 OK 为 false，则 Delay 返回 InfDuration，并且
// Cancel 不执行任何操作。
func (r *Reservation) OK() bool {
	return r.ok
}

// Delay is shorthand for DelayFrom(time.Now()).
func (r *Reservation) Delay() time.Duration {
	return r.DelayFrom(time.Now())
}

// InfDuration 是当 Reservation 不正常时 Delay 返回的持续时间。
const InfDuration = time.Duration(math.MaxInt64)

// 计算t到r.timeToAct（预定时间） 的延迟时间
// DelayFrom 返回预留持有者在执行预留操作之前必须等待的时长。
// 时长为零表示立即执行。
// InfDuration 表示限制器无法在最大等待时间内授予此预留中请求的令牌。
func (r *Reservation) DelayFrom(t time.Time) time.Duration {
	if !r.OK() {
		// 预定无效，返回超长的一个等待时间，表明无法预约执行
		// false，
		return InfDuration
	}
	delay := r.timeToAct.Sub(t)
	if delay < 0 {
		// 预定的时间在t之前，不需要等待
		return 0
	}
	// 预定时间在t之后，返回仍需等待的时间间隔
	return delay
}

func (r *Reservation) Cancel() {
	r.CancelAt(time.Now())
}

// CancelAt 表示预订持有者将不会执行预订操作
// 并尽可能地取消此预订对速率限制的影响，
// 考虑到可能已经进行了其他预订。
// 该函数的功能就是当你的某个预约不需要时，释放资源并取消预约
func (r *Reservation) CancelAt(t time.Time) {
	if !r.OK() {
		// 该预约没有成功
		return
	}

	// 对该预约涉及的限流器开启锁
	r.lim.mu.Lock()
	defer r.lim.mu.Unlock()

	if r.lim.limit == Inf || r.tokens == 0 || r.timeToAct.Before(t) {
		// 限制器没做限速 || 预约的令牌数为0 || 预约时间已经过去
		return // 不需要取消，直接退出
	}

}

func (lim *Limiter) ReserveN(t time.Time, n int) *Re {

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
