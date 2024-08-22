package utils

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type APIUsage struct {
	Count      int64
	LastUpdate time.Time
}

func GetUsage(apiKey string, usageCache *cache.Cache) *APIUsage {
	usagePtr, found := usageCache.Get(apiKey)
	if !found {
		return nil
	}
	return usagePtr.(*APIUsage)
}

func SetUsage(apiKey string, usageCache *cache.Cache, usage *APIUsage, expire bool) {
	if expire {
		usageCache.Set(apiKey, usage, cache.DefaultExpiration)
	} else {
		usageCache.Set(apiKey, usage, cache.NoExpiration)
	}
}

func UpdateUsage(apiKey string, usageCache *cache.Cache, usageMutexMap *sync.Map) {
	mutex := getMutex(apiKey, usageMutexMap)
	mutex.Lock()
	defer mutex.Unlock()

	usage := GetUsage(apiKey, usageCache)
	if usage == nil {
		usage = &APIUsage{Count: 0, LastUpdate: time.Now()}
	}

	usage.Count++
	usage.LastUpdate = time.Now()

	SetUsage(apiKey, usageCache, usage, true)
}

func getMutex(key string, usageMutexMap *sync.Map) *sync.Mutex {
	actualMutex, _ := usageMutexMap.LoadOrStore(key, &sync.Mutex{})
	return actualMutex.(*sync.Mutex)
}

func IncrementAPIUsage(apiKey string, limit int, usageCache *cache.Cache, usageMutexMap *sync.Map) bool {
	// Retrieve the mutex for the specified API key
	usageMutex := getMutex(apiKey, usageMutexMap)

	// Lock the mutex to ensure exclusive access to the usage value for this API key
	usageMutex.Lock()
	defer usageMutex.Unlock()

	// Load the usage for the API key
	usage := GetUsage(apiKey, usageCache)
	if usage == nil {
		// Initialize usage if not found
		usage = &APIUsage{Count: 1, LastUpdate: time.Now()}
		SetUsage(apiKey, usageCache, usage, usage.Count == 1)
	} else {
		// Increment the usage count
		if limit != 0 && usage.Count >= int64(limit) {
			return false
		}
		usage.Count++
	}

	// Update the entry in the cache
	//setUsage(apiKey, usage, usage.Count == 1) // If count was 1, then it's an initialization
	return true
}
