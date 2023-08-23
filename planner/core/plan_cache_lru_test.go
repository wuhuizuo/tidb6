// Package core Copyright 2022 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package core

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wuhuizuo/tidb6/parser/mysql"
	"github.com/wuhuizuo/tidb6/types"
	"github.com/wuhuizuo/tidb6/util/hack"
	"github.com/wuhuizuo/tidb6/util/kvcache"
)

func randomPlanCacheKey() *planCacheKey {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &planCacheKey{
		database:      strconv.FormatInt(int64(random.Int()), 10),
		schemaVersion: time.Now().UnixNano(),
	}
}

func randomPlanCacheValue(types []*types.FieldType) *PlanCacheValue {
	plans := []Plan{&Insert{}, &Update{}, &Delete{}, &PhysicalTableScan{}, &PhysicalTableDual{}, &PhysicalTableReader{},
		&PhysicalTableScan{}, &PhysicalIndexJoin{}, &PhysicalIndexHashJoin{}, &PhysicalIndexMergeJoin{}, &PhysicalIndexMergeReader{},
		&PhysicalIndexLookUpReader{}, &PhysicalApply{}, &PhysicalApply{}, &PhysicalLimit{}}
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &PlanCacheValue{
		Plan:       plans[random.Int()%len(plans)],
		ParamTypes: types,
	}
}

func TestLRUPCPut(t *testing.T) {
	// test initialize
	lruA := NewLRUPlanCache(0, 0, 0, PickPlanFromBucket, MockContext())
	require.Equal(t, lruA.capacity, uint(100))

	maxMemDroppedKv := make(map[kvcache.Key]kvcache.Value)
	lru := NewLRUPlanCache(3, 0, 0, PickPlanFromBucket, MockContext())
	lru.onEvict = func(key kvcache.Key, value kvcache.Value) {
		maxMemDroppedKv[key] = value
	}
	require.Equal(t, uint(3), lru.capacity)

	keys := make([]*planCacheKey, 5)
	vals := make([]*PlanCacheValue, 5)
	pTypes := [][]*types.FieldType{{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeEnum)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDate)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeLong)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeInt24)},
	}

	// one key corresponding to multi values
	for i := 0; i < 5; i++ {
		keys[i] = &planCacheKey{database: strconv.FormatInt(int64(1), 10)}
		vals[i] = &PlanCacheValue{
			ParamTypes: pTypes[i],
		}
		lru.Put(keys[i], vals[i], pTypes[i])
	}
	require.Equal(t, lru.size, lru.capacity)
	require.Equal(t, uint(3), lru.size)

	// test for non-existent elements
	require.Len(t, maxMemDroppedKv, 2)
	for i := 0; i < 2; i++ {
		bucket, exist := lru.buckets[string(hack.String(keys[i].Hash()))]
		require.True(t, exist)
		for element := range bucket {
			require.NotEqual(t, vals[i], element.Value.(*planCacheEntry).PlanValue)
		}
		require.Equal(t, vals[i], maxMemDroppedKv[keys[i]])
	}

	// test for existent elements
	root := lru.lruList.Front()
	require.NotNil(t, root)
	for i := 4; i >= 2; i-- {
		entry, ok := root.Value.(*planCacheEntry)
		require.True(t, ok)
		require.NotNil(t, entry)

		// test key
		key := entry.PlanKey
		require.NotNil(t, key)
		require.Equal(t, keys[i], key)

		bucket, exist := lru.buckets[string(hack.String(keys[i].Hash()))]
		require.True(t, exist)
		element, exist := lru.pickFromBucket(bucket, pTypes[i])
		require.NotNil(t, element)
		require.True(t, exist)
		require.Equal(t, root, element)

		// test value
		value, ok := entry.PlanValue.(*PlanCacheValue)
		require.True(t, ok)
		require.Equal(t, vals[i], value)

		root = root.Next()
	}

	// test for end of double-linked list
	require.Nil(t, root)
}

func TestLRUPCGet(t *testing.T) {
	lru := NewLRUPlanCache(3, 0, 0, PickPlanFromBucket, MockContext())

	keys := make([]*planCacheKey, 5)
	vals := make([]*PlanCacheValue, 5)
	pTypes := [][]*types.FieldType{{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeEnum)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDate)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeLong)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeInt24)},
	}
	// 5 bucket
	for i := 0; i < 5; i++ {
		keys[i] = &planCacheKey{database: strconv.FormatInt(int64(i%4), 10)}
		vals[i] = &PlanCacheValue{ParamTypes: pTypes[i]}
		lru.Put(keys[i], vals[i], pTypes[i])
	}

	// test for non-existent elements
	for i := 0; i < 2; i++ {
		value, exists := lru.Get(keys[i], pTypes[i])
		require.False(t, exists)
		require.Nil(t, value)
	}

	for i := 2; i < 5; i++ {
		value, exists := lru.Get(keys[i], pTypes[i])
		require.True(t, exists)
		require.NotNil(t, value)
		require.Equal(t, vals[i], value)
		require.Equal(t, uint(3), lru.size)
		require.Equal(t, uint(3), lru.capacity)

		root := lru.lruList.Front()
		require.NotNil(t, root)

		entry, ok := root.Value.(*planCacheEntry)
		require.True(t, ok)
		require.Equal(t, keys[i], entry.PlanKey)

		value, ok = entry.PlanValue.(*PlanCacheValue)
		require.True(t, ok)
		require.Equal(t, vals[i], value)
	}
}

func TestLRUPCDelete(t *testing.T) {
	lru := NewLRUPlanCache(3, 0, 0, PickPlanFromBucket, MockContext())

	keys := make([]*planCacheKey, 3)
	vals := make([]*PlanCacheValue, 3)
	pTypes := [][]*types.FieldType{{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeEnum)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDate)},
	}
	for i := 0; i < 3; i++ {
		keys[i] = &planCacheKey{database: strconv.FormatInt(int64(i), 10)}
		vals[i] = &PlanCacheValue{ParamTypes: pTypes[i]}
		lru.Put(keys[i], vals[i], pTypes[i])
	}
	require.Equal(t, 3, int(lru.size))

	lru.Delete(keys[1])
	value, exists := lru.Get(keys[1], pTypes[1])
	require.False(t, exists)
	require.Nil(t, value)
	require.Equal(t, 2, int(lru.size))

	_, exists = lru.Get(keys[0], pTypes[0])
	require.True(t, exists)

	_, exists = lru.Get(keys[2], pTypes[2])
	require.True(t, exists)
}

func TestLRUPCDeleteAll(t *testing.T) {
	lru := NewLRUPlanCache(3, 0, 0, PickPlanFromBucket, MockContext())

	keys := make([]*planCacheKey, 3)
	vals := make([]*PlanCacheValue, 3)
	pTypes := [][]*types.FieldType{{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeEnum)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDate)},
	}
	for i := 0; i < 3; i++ {
		keys[i] = &planCacheKey{database: strconv.FormatInt(int64(i), 10)}
		vals[i] = &PlanCacheValue{ParamTypes: pTypes[i]}
		lru.Put(keys[i], vals[i], pTypes[i])
	}
	require.Equal(t, 3, int(lru.size))

	lru.DeleteAll()

	for i := 0; i < 3; i++ {
		value, exists := lru.Get(keys[i], pTypes[i])
		require.False(t, exists)
		require.Nil(t, value)
		require.Equal(t, 0, int(lru.size))
	}
}

func TestLRUPCSetCapacity(t *testing.T) {
	maxMemDroppedKv := make(map[kvcache.Key]kvcache.Value)
	lru := NewLRUPlanCache(5, 0, 0, PickPlanFromBucket, MockContext())
	lru.onEvict = func(key kvcache.Key, value kvcache.Value) {
		maxMemDroppedKv[key] = value
	}
	require.Equal(t, uint(5), lru.capacity)

	keys := make([]*planCacheKey, 5)
	vals := make([]*PlanCacheValue, 5)
	pTypes := [][]*types.FieldType{{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeEnum)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDate)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeLong)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeInt24)},
	}

	// one key corresponding to multi values
	for i := 0; i < 5; i++ {
		keys[i] = &planCacheKey{database: strconv.FormatInt(int64(1), 10)}
		vals[i] = &PlanCacheValue{ParamTypes: pTypes[i]}
		lru.Put(keys[i], vals[i], pTypes[i])
	}
	require.Equal(t, lru.size, lru.capacity)
	require.Equal(t, uint(5), lru.size)

	err := lru.SetCapacity(3)
	require.NoError(t, err)

	// test for non-existent elements
	require.Len(t, maxMemDroppedKv, 2)
	for i := 0; i < 2; i++ {
		bucket, exist := lru.buckets[string(hack.String(keys[i].Hash()))]
		require.True(t, exist)
		for element := range bucket {
			require.NotEqual(t, vals[i], element.Value.(*planCacheEntry).PlanValue)
		}
		require.Equal(t, vals[i], maxMemDroppedKv[keys[i]])
	}

	// test for existent elements
	root := lru.lruList.Front()
	require.NotNil(t, root)
	for i := 4; i >= 2; i-- {
		entry, ok := root.Value.(*planCacheEntry)
		require.True(t, ok)
		require.NotNil(t, entry)

		// test value
		value, ok := entry.PlanValue.(*PlanCacheValue)
		require.True(t, ok)
		require.Equal(t, vals[i], value)

		root = root.Next()
	}

	// test for end of double-linked list
	require.Nil(t, root)

	err = lru.SetCapacity(0)
	require.Error(t, err, "capacity of LRU cache should be at least 1")
}

func TestIssue37914(t *testing.T) {
	lru := NewLRUPlanCache(3, 0.1, 1, PickPlanFromBucket, MockContext())

	pTypes := []*types.FieldType{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)}
	key := &planCacheKey{database: strconv.FormatInt(int64(1), 10)}
	val := &PlanCacheValue{ParamTypes: pTypes}

	require.NotPanics(t, func() {
		lru.Put(key, val, pTypes)
	})
}

func TestIssue38244(t *testing.T) {
	lru := NewLRUPlanCache(3, 0, 0, PickPlanFromBucket, MockContext())
	require.Equal(t, uint(3), lru.capacity)

	keys := make([]*planCacheKey, 5)
	vals := make([]*PlanCacheValue, 5)
	pTypes := [][]*types.FieldType{{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeEnum)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDate)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeLong)},
		{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeInt24)},
	}

	// one key corresponding to multi values
	for i := 0; i < 5; i++ {
		keys[i] = &planCacheKey{database: strconv.FormatInt(int64(i), 10)}
		vals[i] = &PlanCacheValue{ParamTypes: pTypes[i]}
		lru.Put(keys[i], vals[i], pTypes[i])
	}
	require.Equal(t, lru.size, lru.capacity)
	require.Equal(t, uint(3), lru.size)
	require.Equal(t, len(lru.buckets), 3)
}

func TestLRUPlanCacheMemoryUsage(t *testing.T) {
	pTypes := []*types.FieldType{types.NewFieldType(mysql.TypeFloat), types.NewFieldType(mysql.TypeDouble)}
	ctx := MockContext()
	ctx.GetSessionVars().EnablePreparedPlanCacheMemoryMonitor = true
	lru := NewLRUPlanCache(3, 0, 0, PickPlanFromBucket, ctx)
	evict := make(map[kvcache.Key]kvcache.Value)
	lru.onEvict = func(key kvcache.Key, value kvcache.Value) {
		evict[key] = value
	}
	var res int64 = 0
	// put
	for i := 0; i < 3; i++ {
		k := randomPlanCacheKey()
		v := randomPlanCacheValue(pTypes)
		lru.Put(k, v, pTypes)
		res += k.MemoryUsage() + v.MemoryUsage()
		require.Equal(t, lru.MemoryUsage(), res)
	}
	// evict
	p := &PhysicalTableScan{}
	k := &planCacheKey{database: "3"}
	v := &PlanCacheValue{Plan: p}
	lru.Put(k, v, pTypes)
	res += k.MemoryUsage() + v.MemoryUsage()
	for kk, vv := range evict {
		res -= kk.(*planCacheKey).MemoryUsage() + vv.(*PlanCacheValue).MemoryUsage()
	}
	require.Equal(t, lru.MemoryUsage(), res)
	// delete
	lru.Delete(k)
	res -= k.MemoryUsage() + v.MemoryUsage()
	require.Equal(t, lru.MemoryUsage(), res)
	// delete all
	lru.DeleteAll()
	require.Equal(t, lru.MemoryUsage(), int64(0))
}
