package performance

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/telark/exporter/internal/constants"
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
	// A write landed while this list was being built: its generation is gone and
	// the blob would only sit in Redis until its TTL.
	if cacheKey != clh.cacheKey(r) {
		return
	}
	// One level deep: this validates the body once and keeps the data field's
	// bytes without ever building the value tree of a multi-megabyte list.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(responseCapture.body, &envelope); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrOptimizerCacheStoreError), requestID, err))
		return
	}
	blob := responseCapture.body
	if data, exists := envelope[constants.ResponseDataField]; exists {
		blob = data
	}
	clh.localPut(cacheKey, blob)
	clh.optimizer.SetTTL(cacheKey, blob, clh.storeTTL())
}

func (clh *CachedListHandler) storeTTL() time.Duration {
	if clh.operation == constants.OpGet {
		return constants.GetCacheTTL
	}
	return constants.CacheTTL
}
