package performance

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/base"
)

func generateRequestID() string {
	return time.Now().Format(RequestIDFormat) + "-" + randomString(RequestIDLength)
}

func randomString(length int) string {
	if length <= constants.DefaultInitValue {
		return constants.EmptyString
	}
	b := make([]byte, length)
	maxVal := big.NewInt(int64(len(Charset)))
	for i := range length {
		n, err := rand.Int(rand.Reader, maxVal)
		if err != nil {
			b[i] = Charset[constants.DefaultInitValue]
			continue
		}
		b[i] = Charset[n.Int64()]
	}
	return string(b)
}

func storeResponseInCache(clh *CachedListHandler, r *http.Request, responseCapture *responseCaptureWriter, cacheKey string, requestID string) {
	var responseMap map[string]any
	if err := json.Unmarshal(responseCapture.body, &responseMap); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrOptimizerCacheStoreError), requestID, err))
		return
	}

	lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheStoreSuccess), requestID, cacheKey, responseCapture.statusCode))

	if data, exists := responseMap["data"]; exists {
		storeDataInCache(clh, r, data, cacheKey)
	} else {
		clh.optimizer.Set(cacheKey, responseCapture.body)
		lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheStoreSuccess), requestID, cacheKey, responseCapture.statusCode))
	}
}

func storeDataInCache(clh *CachedListHandler, r *http.Request, data any, cacheKey string) {
	if isGetOperation(r) {
		storeGetOperationData(clh, data, cacheKey)
	} else {
		clh.optimizer.Set(cacheKey, data)
	}
}

func storeGetOperationData(clh *CachedListHandler, data any, cacheKey string) {
	if resourceVersion := extractResourceVersion(data); resourceVersion != "" {
		versionedCacheKey := cacheKey + ":" + resourceVersion
		clh.optimizer.Set(versionedCacheKey, data)
		clh.optimizer.Set(cacheKey, data)
		lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheStoreVersionedKey), versionedCacheKey, cacheKey))
	} else {
		if b, err := json.Marshal(data); err == nil {
			clh.optimizer.Set(cacheKey, b)
		} else {
			clh.optimizer.Set(cacheKey, data)
		}
		lg.Info(fmt.Sprintf(string(constants.InfOptimizerCacheStoreRegularKey), cacheKey))
	}
}

func isGetOperation(r *http.Request) bool {
	return r.Method == string(base.Get)
}

func extractResourceVersion(data any) string {
	if dataMap, ok := data.(map[string]any); ok {
		if metadata, exists := dataMap["metadata"].(map[string]any); exists {
			if resourceVersion, ok := metadata["resourceVersion"].(string); ok {
				return resourceVersion
			}
		}
	}
	return constants.EmptyString
}
