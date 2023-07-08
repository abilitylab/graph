package graph

import (
	"log"
	"sync"

	hnswgo "github.com/abilitylab/graph/pkg/hnsw"
)

type SpaceType string

const (
	SpaceTypeIP     SpaceType = "ip"
	SpaceTypeCosine SpaceType = "cosine"
	SpaceTypeL2     SpaceType = "l2"
)

type Configuration struct {
	Dim            int
	M              int
	EFConstruction int
	EF             int
	MaxElements    uint32
	SpaceType      SpaceType
}

type Service struct {
	dim           int
	h             *hnswgo.HNSW
	nextIndex     uint32
	labelInnerMap map[string]uint32
	labelOuterMap map[uint32]string
	rwMtx         sync.RWMutex
}

func New(cfg *Configuration) *Service {
	h := hnswgo.New(
		cfg.Dim,
		cfg.M,
		cfg.EFConstruction,
		100,
		cfg.MaxElements,
		string(cfg.SpaceType))

	return &Service{
		dim:           cfg.Dim,
		h:             h,
		nextIndex:     0,
		labelInnerMap: make(map[string]uint32, cfg.MaxElements),
		labelOuterMap: make(map[uint32]string, cfg.MaxElements),
		rwMtx:         sync.RWMutex{},
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

func (s *Service) SetEF(ef int) {
	s.h.SetEf(ef)
}

func (s *Service) Put(outerLabel string, vector []float32) {
	if len(vector) != s.dim {
		panic("put: vector length is not equal to dim")
	}

	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	innerLabel, found := s.findInnerLabelUnsafe(outerLabel)
	if !found {
		innerLabel = s.createNewLabelUnSafe(outerLabel)
	}

	// fmt.Println("put: adding outerLabel:", outerLabel)

	s.h.AddPoint(vector, innerLabel)

	// fmt.Println("put: added outerLabel:", outerLabel)
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

func (s *Service) Search(vectors []float32, resultsNum int) map[string]float32 {
	if len(vectors) != s.dim {
		log.Println("search: vector length is not equal to dim")
		return nil
	}

	s.rwMtx.RLock()
	defer s.rwMtx.RUnlock()

	innerLabels, distances := s.h.SearchKNN(vectors, resultsNum)

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
