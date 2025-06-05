package limiter

import (
	"math"
	"time"
)

// Limit defines the maximum frequency of some events.
// Limit is represented as number of events per second.
// A zero Limit allows no events.
type Limit float64

// Inf is the infinite rate limit; it allows all events (even if burst is zero).
const Inf = Limit(math.MaxFloat64)

// tokensFromDuration 是一个从时间长度到令牌数量的单位转换函数
// 令牌数量可以以每秒限制令牌数量的速率在该持续时间内累积。
func (limit Limit) tokensFormDuration(d time.Duration) float64 {
	if limit <= 0 {
		return 0
	}
	return d.Seconds() * float64(limit)
}

// durationFromTokens 是一个单位转换函数，
// 用于将令牌数量转换为以每秒限制令牌数的速率累积令牌所需的时间。
func (limit Limit) durationFormTokens(tokens float64) time.Duration {
	if limit <= 0 {
		return InfDuration
	}
	duration := (tokens / float64(limit)) * float64(time.Second)

	//将持续时间限制为最大可表示的 int64 值，以避免溢出。
	if duration > float64(math.MaxInt64) {
		return InfDuration
	}
	return time.Duration(duration)
}

// InfDuration is the duration returned by Delay when a Reservation is not OK.
const InfDuration = time.Duration(math.MaxInt64)
