package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/abilitylab/graph/pkg/graph"
	"github.com/abilitylab/graph/pkg/inmemory"
	"github.com/abilitylab/logger"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	logger2 "gorm.io/gorm/logger"
)

var (
	duration    = flag.Duration("duration", 0, "maximum duration to calculate feed vectors")
	reloadEvery = flag.Duration("reload-every", 0, "reload every duration")
)

const hnswEnabled = true
const dim = 768
const M = 40

const maxElements = 5000000

const putPostsWorkerCount = 12

var (
	hnswGraph     *graph.Service
	inMemoryGraph *inmemory.Service

	postLabels    = make(map[string]map[string]struct{}, maxElements)
	postLabelsMtx sync.RWMutex
)

var (
	db *gorm.DB
)

func init() {
	var err error
	db, err = gorm.Open(mysql.Open(constant.DatabaseDSN), &gorm.Config{
		Logger: logger2.Default.LogMode(logger2.Silent),
	})
	if err != nil {
		logger.Fatal("database open failed", zap.Error(err))
	}

	// model.AutoMigrateAll(db)

	flag.Parse()

	logger.Info("set duration", zap.Duration("duration", *duration))
	logger.Info("set reload every", zap.Duration("reload-every", *reloadEvery))
}

const defaultMaxDistance = 0.7
const defaultMinDistance = -1.0

func main() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	if *reloadEvery > 0 {
		go func() {
			time.Sleep(*reloadEvery)
			os.Exit(0)
		}()
	}

	if hnswEnabled {
		hnswGraph = graph.New(&graph.Configuration{
			Dim:            dim,
			M:              M, // original value is 16
			EFConstruction: 50,
			EF:             200,
			MaxElements:    maxElements,
			SpaceType:      graph.SpaceTypeCosine,
		})
	}

	inMemoryGraph = inmemory.New(&inmemory.Configuration{
		Dim:         dim,
		MaxElements: maxElements,
		SpaceType:   graph.SpaceTypeCosine,
	})

	go runGraph()

	e := echo.New()
	e.Use(middleware.Recover())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.GET("/info", func(c echo.Context) error {
		return c.String(http.StatusOK, fmt.Sprintf("Indexes loaded: %d", inMemoryGraph.IndexesLoaded()))
	})
	e.GET("/list-ids", func(c echo.Context) error {
		return c.JSON(http.StatusOK, inMemoryGraph.ListIDs())
	})
	e.POST("/search", func(c echo.Context) error {
		vectorStr := c.FormValue("vector")
		if vectorStr == "" {
			return c.String(http.StatusBadRequest, "vector must be set")
		}

		var vector []float32

		err := json.Unmarshal([]byte(vectorStr), &vector)
		if err != nil {
			return c.String(http.StatusBadRequest, "vector must be valid json")
		}

		resultsNum := 100

		resultsStr := c.FormValue("results")
		if resultsStr != "" {
			var err error
			resultsNum, err = strconv.Atoi(resultsStr)
			if err != nil {
				return c.String(http.StatusBadRequest, "results must be a number")
			}
		}

		exactStr := c.FormValue("exact")
		var exact []string
		if exactStr != "" {
			err = json.Unmarshal([]byte(exactStr), &exact)
			if err != nil {
				logger.Error("exact must be valid json", zap.Error(err), zap.String("exact", exactStr))
				return c.String(http.StatusBadRequest, "exact must be valid json")
			}
		}

		maxDistanceStr := c.FormValue("maxDistance")
		var maxDistance float32 = defaultMaxDistance
		if maxDistanceStr != "" {
			newMaxDistance, err := strconv.ParseFloat(maxDistanceStr, 32)
			if err != nil {
				logger.Error("maxDistance must be a number", zap.Error(err), zap.String("maxDistance", maxDistanceStr))
				return c.String(http.StatusBadRequest, "defaultMaxDistance must be a number")
			}
			maxDistance = float32(newMaxDistance)
		}

		minDistanceStr := c.FormValue("minDistance")
		var minDistance float32 = defaultMinDistance
		if minDistanceStr != "" {
			newMinDistance, err := strconv.ParseFloat(minDistanceStr, 32)
			if err != nil {
				logger.Error("minDistance must be a number", zap.Error(err), zap.String("minDistance", minDistanceStr))
				return c.String(http.StatusBadRequest, "minDistance must be a number")
			}
			minDistance = float32(newMinDistance)
		}

		filterCategory := c.FormValue("category")

		if len(vector) == 0 && len(exact) == 0 {
			return c.String(http.StatusBadRequest, "vector or exact must be set")
		}
		if len(vector) != dim {
			return c.String(http.StatusBadRequest, "vector must be "+strconv.Itoa(dim)+"-dimensional")
		}

		if len(exact) > 0 || !hnswEnabled {
			contains := make([][]byte, len(exact))
			for i, e := range exact {
				contains[i] = []byte(e)
			}
			return c.JSON(http.StatusOK, filterResults(inMemoryGraph.Search(contains, vector, resultsNum), filterCategory))
		}

		// fmt.Println("searching hnsw")

		results := hnswGraph.Search(vector, resultsNum)
		for key, distance := range results {
			if distance > maxDistance || distance < minDistance {
				delete(results, key)
			}
		}

		return c.JSON(http.StatusOK, filterResults(results, filterCategory))
	})
	e.Logger.Fatal(e.Start("0.0.0.0:8080"))
}

func runGraph() {
	ch := make(chan *model.Article, 4000)

	go startGraphLoader(ch)

	var counter atomic.Int64

	wg := &sync.WaitGroup{}
	for i := 0; i < putPostsWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for c := range ch {
				if curr := counter.Load(); curr > maxElements-10 {
					logger.Error("too many elements", zap.Int64("elements", curr))
					continue
				}

				if c.Vector == nil && c.VectorDB == "" {
					continue
				} else if c.Vector != nil {
					var err error
					c.Vector, err = model.StringToVector(c.VectorDB)
					if err != nil {
						logger.Error("error converting vector", zap.Error(err), zap.String("vector", c.VectorDB))
						continue
					}
				}

				if *duration > 0 && time.Since(c.CreatedAt) > *duration {
					continue
				}

				// if c.PublishDate == nil || c.PublishDate.After(*end) || c.Post.PublishedAt < *start || c.Type != journal.ActionCreate {
				// 	continue
				// }

				// if c.Post.Language != *language {
				// 	continue
				// }

				err := putPost(c)
				if err != nil {
					logger.Error("error putting post", zap.Error(err))
				}

				var act = counter.Inc()
				if act%10000 == 0 {
					logger.Info("posts processed", zap.Int64("count", act))
				}
			}
		}()
	}
	wg.Wait()

	log.Println("loading graph stopped!")
}

func putPost(sp *model.Article) error {
	if sp == nil || sp.Vector == nil {
		return nil
	}

	hashid := setup.ArticleHashID.Encode(sp.ID)

	vector := reduceFloat(sp.Vector)

	if hnswEnabled {
		hnswGraph.Put(hashid, vector)
	}

	inMemoryGraph.Put(hashid, bytes.ToLower([]byte(sp.Title+" "+sp.Summary)), vector)

	putLabels(hashid, sp.Categories)

	return nil
}

func reduceFloat(v []float64) []float32 {
	var result = make([]float32, len(v))
	for i, f := range v {
		result[i] = float32(f)
	}
	return result
}

func startGraphLoader(ch chan *model.Article) {
	var lastMinID, lastMaxID uint64 = math.MaxUint64, 0

	for {
		articles, err := qSrv.GetVectorizedNotLoadedArticles(db, 400, lastMinID, lastMaxID)
		if err != nil {
			logger.Error("error getting not processed links", zap.Error(err))
			time.Sleep(time.Second * 20)
			continue
		} else if len(articles) == 0 {
			time.Sleep(time.Second * 20)
			continue
		}

		for _, link := range articles {
			if link.ID < lastMinID {
				lastMinID = link.ID
			}
			if link.ID > lastMaxID {
				lastMaxID = link.ID
			}

			ch <- link
		}
	}
}

func putLabels(post string, labels []string) {
	if len(labels) == 0 {
		return
	}

	newLabels := make(map[string]struct{})
	for _, label := range labels {
		newLabels[label] = struct{}{}
	}

	postLabelsMtx.Lock()
	defer postLabelsMtx.Unlock()

	postLabels[post] = newLabels
}

func filterResults(results map[string]float32, label string) map[string]float32 {
	if label == "" {
		return results
	}

	filtered := make(map[string]float32, len(results))
	for post, value := range results {
		if postHasLabel(post, label) {
			filtered[post] = value
		}
	}
	return filtered
}

func postHasLabel(post, label string) bool {
	postLabelsMtx.RLock()
	defer postLabelsMtx.RUnlock()

	if labels, ok := postLabels[post]; ok {
		_, ok2 := labels[label]
		return ok2
	}

	return false
}
