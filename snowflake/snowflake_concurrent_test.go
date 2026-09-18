package snowflake

import (
	"sync"
	"testing"
)

// TestConcurrentGenerateUnique 多 goroutine 并发 ID 生成唯一性（配合 -race）。
// Generate 持有节点锁，预期：不重复、无数据竞争。
func TestConcurrentGenerateUnique(t *testing.T) {
	node, err := NewNode(1)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}

	const goroutines = 8
	const perGoroutine = 2000

	ids := make([][]ID, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = make([]ID, 0, perGoroutine)
			for j := 0; j < perGoroutine; j++ {
				ids[idx] = append(ids[idx], node.Generate())
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[ID]struct{}, goroutines*perGoroutine)
	for _, batch := range ids {
		for _, id := range batch {
			if _, dup := seen[id]; dup {
				t.Fatalf("并发生成出现重复 ID: %d", id)
			}
			seen[id] = struct{}{}
		}
	}
}

// TestConcurrentNewNodeRace 检测 NewNode 与 Generate/ID 解析并发执行时的数据竞争。
// NewNode 会无锁改写包级全局位运算参数（nodeMask/stepMask/timeShift 等），
// 而 Generate 与 ID.Node()/ID.Time() 读取这些全局量，-race 下预期报警。
// 该测试用于暴露问题：正常运行时应避免并发调用 NewNode。
func TestConcurrentNewNodeRace(t *testing.T) {
	node, err := NewNode(1)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}

	stop := make(chan struct{})
	var rebuildWG sync.WaitGroup

	// 一方持续重建设点（改写全局参数）
	rebuildWG.Add(1)
	go func() {
		defer rebuildWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := NewNode(3); err != nil {
				t.Errorf("NewNode: %v", err)
				return
			}
		}
	}()

	// 另一方持续生成并解析 ID（读取全局参数）
	var genWG sync.WaitGroup
	for i := 0; i < 4; i++ {
		genWG.Add(1)
		go func() {
			defer genWG.Done()
			for j := 0; j < 20000; j++ {
				id := node.Generate()
				_ = id.Node()
				_ = id.Time()
				_ = id.Step()
			}
		}()
	}

	genWG.Wait()
	close(stop)
	rebuildWG.Wait()
}
