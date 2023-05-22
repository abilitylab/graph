package vector

import (
	"math"

	"github.com/chewxy/math32"
)

func Cosine(a, b []float64) (cosine float64) {
	length := len(a)
	if length != len(b) {
		// return 0.0, errors.New("vectors are not the same length")
		panic("cosine: vectors are not the same length")
	}
	sumA, s1, s2 := 0.0, 0.0, 0.0
	for k := 0; k < length; k++ {
		sumA += a[k] * b[k]
		s1 += math.Pow(a[k], 2)
		s2 += math.Pow(b[k], 2)
	}
	if s1 == 0 || s2 == 0 {
		return 0.0
	}
	return sumA / (math.Sqrt(s1) * math.Sqrt(s2))
}

func Cosine32(a, b []float32) (cosine float32) {
	length := len(a)
	if length != len(b) {
		// return 0.0, errors.New("vectors are not the same length")
		panic("cosine32: vectors are not the same length")
	}
	sumA, s1, s2 := float32(0.0), float32(0.0), float32(0.0)
	for k := 0; k < length; k++ {
		sumA += a[k] * b[k]
		s1 += math32.Pow(a[k], 2)
		s2 += math32.Pow(b[k], 2)
	}
	if s1 == 0 || s2 == 0 {
		return 0.0
	}
	return sumA / (math32.Sqrt(s1) * math32.Sqrt(s2))
}

// func Cosine32Diff(a []float32, b []float32) (cosine float32, err error) {
//	count := 0
//	lengthA, lengthB := len(a), len(b)
//	if lengthA > lengthB {
//		count = lengthA
//	} else {
//		count = lengthB
//	}
//	sumA, s1, s2 := float32(0.0), float32(0.0), float32(0.0)
//	for k := 0; k < count; k++ {
//		if k >= lengthA {
//			s2 += math32.Pow(b[k], 2)
//			continue
//		}
//		if k >= lengthB {
//			s1 += math32.Pow(a[k], 2)
//			continue
//		}
//		sumA += a[k] * b[k]
//		s1 += math32.Pow(a[k], 2)
//		s2 += math32.Pow(b[k], 2)
//	}
//	if s1 == 0 || s2 == 0 {
//		return 0.0, errors.New("vectors should not be null (all zeros)")
//	}
//	return sumA / (math32.Sqrt(s1) * math32.Sqrt(s2)), nil
// }
