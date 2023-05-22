package inmemory

import (
	"bytes"
	"log"
	"sort"

	vectormath "github.com/abilitylab/graph/pkg/vector"
)

type point struct {
	text   []byte
	vector []float32
}

func (s *Service) addPoint(text []byte, vector []float32, innerLabel uint32) error {
	s.points[innerLabel] = &point{
		text:   text,
		vector: vector,
	}

	return nil
}

const maxDistance = 0.6

func (s *Service) searchPoint(contains [][]byte, vector []float32, resultsNum int) ([]uint32, []float32) {
	var (
		distancesMap = make(map[uint32]float32, resultsNum)
		innerLabels  = make([]uint32, 0, resultsNum)
		distances    = make([]float32, 0, resultsNum)
	)

	// TODO: parallelize this:

	for innerLabel, point := range s.points {
		if point == nil {
			log.Println("searchPoint: point is nil")
			continue
		}

		matches := true

		if len(vector) == 0 {
			distancesMap[innerLabel] = 0.5
			continue
		} else if len(contains) > 0 {
			for _, contain := range contains {
				if !bytes.Contains(point.text, contain) {
					matches = false
					break
				}
			}
		}

		if matches {
			dist := 1.0 - vectormath.Cosine32(vector, point.vector)
			if dist <= maxDistance {
				distancesMap[innerLabel] = dist
			}
		}
	}

	var sortedMap = make([]uint32, len(distancesMap))
	i := 0
	for innerLabel := range distancesMap {
		sortedMap[i] = innerLabel
		i++
	}
	sort.Slice(sortedMap, func(i, j int) bool {
		return distancesMap[sortedMap[i]] < distancesMap[sortedMap[j]]
	})

	if len(sortedMap) > resultsNum { // TODO: this is limiting the results to the first resultsNum if we only use exact matches
		sortedMap = sortedMap[:resultsNum]
	}

	for _, key := range sortedMap {
		innerLabels = append(innerLabels, key)
		distances = append(distances, distancesMap[key])
	}

	return innerLabels, distances
}
