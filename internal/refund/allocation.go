package refund

import (
	"math/big"
	"sort"
)

const maxAmount = int64(9007199254740991)

// Allocate conserves whole Rupiah exactly using stable largest-remainder allocation.
func Allocate(total int64, w []int64, keys []string) ([]int64, error) {
	if total < 0 || total > maxAmount || len(w) == 0 || len(w) != len(keys) {
		return nil, ErrValidation
	}
	sum := new(big.Int)
	for _, v := range w {
		if v < 0 {
			return nil, ErrValidation
		}
		sum.Add(sum, big.NewInt(v))
	}
	if sum.Sign() == 0 {
		return nil, ErrValidation
	}
	out := make([]int64, len(w))
	rem := make([]*big.Int, len(w))
	ids := make([]int, len(w))
	left := total
	for i, v := range w {
		n := new(big.Int).Mul(big.NewInt(total), big.NewInt(v))
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(n, sum, r)
		out[i] = q.Int64()
		left -= out[i]
		rem[i] = r
		ids[i] = i
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
		if c := rem[a].Cmp(rem[b]); c != 0 {
			return c > 0
		}
		return keys[a] < keys[b]
	})
	for i := int64(0); i < left; i++ {
		out[ids[i]]++
	}
	return out, nil
}

// component refunds successive units from a fixed line budget without rounding drift.
func component(b int64, k, q, n int) int64 {
	a := new(big.Int).Mul(big.NewInt(b), big.NewInt(int64(k+q)))
	a.Quo(a, big.NewInt(int64(n)))
	c := new(big.Int).Mul(big.NewInt(b), big.NewInt(int64(k)))
	c.Quo(c, big.NewInt(int64(n)))
	return a.Sub(a, c).Int64()
}
