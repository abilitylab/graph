package inmemory

import (
	"bytes"
	"log"
	"sync"

	"github.com/abilitylab/graph/pkg/graph"
)

type Configuration struct {
	Dim         int
	MaxElements uint32
	SpaceType   graph.SpaceType
}

type Service struct {
	dim           int
	nextIndex     uint32
	labelInnerMap map[string]uint32
	labelOuterMap map[uint32]string
	rwMtx         sync.RWMutex
	points        map[uint32]*point
}

func New(cfg *Configuration) *Service {
	return &Service{
		dim:           cfg.Dim,
		nextIndex:     0,
		labelInnerMap: make(map[string]uint32, cfg.MaxElements),
		labelOuterMap: make(map[uint32]string, cfg.MaxElements),
		rwMtx:         sync.RWMutex{},
		points:        make(map[uint32]*point, cfg.MaxElements),
	}
}

func (s *Service) findInnerLabel(outerLabel string) (uint32, bool) {
	s.rwMtx.RLock()
	defer s.rwMtx.RUnlock()

	return s.findInnerLabelUnsafe(outerLabel)
}

func (s *Service) findInnerLabelUnsafe(outerLabel string) (uint32, bool) {
	ret, found := s.labelInnerMap[outerLabel]
	return ret, found
}

func (s *Service) findOuterLabel(innerLabel uint32) (string, bool) {
	s.rwMtx.RLock()
	defer s.rwMtx.RUnlock()

	return s.findOuterLabelUnsafe(innerLabel)
}

func (s *Service) findOuterLabelUnsafe(innerLabel uint32) (string, bool) {
	ret, found := s.labelOuterMap[innerLabel]
	return ret, found
}

func (s *Service) createNewLabelUnSafe(outerLabel string) uint32 {
	innerLabel := s.nextIndex
	s.nextIndex++

	s.labelOuterMap[innerLabel] = outerLabel
	s.labelInnerMap[outerLabel] = innerLabel

	return innerLabel
}

func (s *Service) Put(outerLabel string, text []byte, vector []float32) {
	if len(vector) != s.dim {
		panic("put: vector length is not equal to dim")
	}

	if len(text) > 1024 {
		text = text[:1024]
	}
	text = bytes.ToLower(text)

	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	innerLabel, found := s.findInnerLabelUnsafe(outerLabel)
	if !found {
		innerLabel = s.createNewLabelUnSafe(outerLabel)
	}

	s.addPoint(text, vector, innerLabel)
}

func (s *Service) ListIDs() []string {
	s.rwMtx.RLock()
	defer s.rwMtx.RUnlock()

	out := make([]string, len(s.labelInnerMap))
	i := 0
	for k := range s.labelInnerMap {
		out[i] = k
		i++
	}
	return out
}

func (s *Service) IndexesLoaded() uint32 {
	return s.nextIndex
}

func (s *Service) Delete(outerLabel string) {
	panic("not implemented")
}

func (s *Service) Search(contains [][]byte, vectors []float32, resultsNum int) map[string]float32 {
	if len(vectors) != s.dim && len(vectors) != 0 {
		log.Println("search: vector length is not equal to dim")
		return nil
	}

	s.rwMtx.RLock()
	defer s.rwMtx.RUnlock()

	innerLabels, distances := s.searchPoint(contains, vectors, resultsNum)

	results := make(map[string]float32, len(innerLabels))

	for i, innerLabel := range innerLabels {
		outerLabel, found := s.findOuterLabelUnsafe(innerLabel)
		if !found {
			panic("outerLabel not found")
		}

		results[outerLabel] = distances[i]
	}

	return results
}
