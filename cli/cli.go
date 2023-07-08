package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	url2 "net/url"
	"sort"
	"time"

	"github.com/abilitylab/logger"
	"go.uber.org/zap"
)

type getSimilarCfg struct {
	minDistance    float32
	minDistanceSet bool
	maxDistance    float32
	maxDistanceSet bool
}

func WithMinDistance(minDistance float32) func(*getSimilarCfg) {
	return func(cfg *getSimilarCfg) {
		cfg.minDistance = minDistance
		cfg.minDistanceSet = true
	}
}

func WithMaxDistance(maxDistance float32) func(*getSimilarCfg) {
	return func(cfg *getSimilarCfg) {
		cfg.maxDistance = maxDistance
		cfg.maxDistanceSet = true
	}
}

func GetSimilar(ctx context.Context, hostPost string, vector []float64, limit int, opts ...func(cfg *getSimilarCfg)) (map[string]float32, error) {
	cfg := &getSimilarCfg{
		minDistance: 0,
		maxDistance: 0.7,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	u := url2.URL{
		Scheme:      "http",
		Opaque:      "",
		User:        nil,
		Host:        hostPost,
		Path:        "/search",
		RawPath:     "",
		ForceQuery:  false,
		RawQuery:    "",
		Fragment:    "",
		RawFragment: "",
	}

	q := u.Query()

	vec, err := VectorToString(vector)
	if err != nil {
		logger.Error("error converting vector to string", zap.Error(err))
		return nil, err
	}

	q.Add("vector", vec)
	q.Add("results", fmt.Sprintf("%d", limit))
	if cfg.minDistanceSet {
		q.Add("minDistance", fmt.Sprintf("%f", cfg.minDistance))
	}
	if cfg.maxDistanceSet {
		q.Add("maxDistance", fmt.Sprintf("%f", cfg.maxDistance))
	}

	u.RawQuery = q.Encode()

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", u.String(), nil)
	if err != nil {
		logger.Error("error creating request", zap.Error(err))
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("error sending request", zap.Error(err))
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		logger.Error("error sending request", zap.Int("status", res.StatusCode))
		return nil, fmt.Errorf("error sending request, status code: %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error("error reading response", zap.Error(err))
		return nil, err
	}

	var r map[string]float32
	err = json.Unmarshal(body, &r)
	if err != nil {
		logger.Error("error unmarshalling response", zap.Error(err))
		return nil, err
	}

	return r, nil
}

func VectorToString(vector []float64) (string, error) {
	if vector == nil {
		return "", nil
	}

	bytes, err := json.Marshal(vector)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func SortSimilar(similar map[string]float32) []string {
	var keys []string
	for k := range similar {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		return similar[keys[i]] < similar[keys[j]]
	})

	return keys
}
