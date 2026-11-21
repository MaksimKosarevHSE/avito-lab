package positions

import (
	standart_heap "container/heap"
	"sync"

	"github.com/google/uuid"
)

type heap []*queueItem

type queueItem struct {
	trip  SelectedTrip
	index int
}

type PriorityQueue struct {
	h        heap
	getByKey map[uuid.UUID]*queueItem
	mutex    sync.Mutex
	notEmpty *sync.Cond
	capacity int
	closed   bool
}

type PushStatus string

type EnqueueResult struct {
	Status  PushStatus
	Removed *SelectedTrip
}

const (
	PushStatusQueued    PushStatus = "queued"
	PushStatusDuplicate PushStatus = "duplicate"
	PushStatusDropped   PushStatus = "dropped"
)

func NewPriorityQueue(capacity int) *PriorityQueue {
	if capacity <= 0 {
		panic("capacity must be positive")
	}

	q := &PriorityQueue{
		h:        make(heap, 0, capacity),
		getByKey: make(map[uuid.UUID]*queueItem),
		capacity: capacity,
	}

	q.notEmpty = sync.NewCond(&q.mutex)

	return q
}

func (q *PriorityQueue) Enqueue(trip SelectedTrip) EnqueueResult {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if _, ok := q.getByKey[trip.TripID]; ok {
		return EnqueueResult{
			Status: PushStatusDuplicate,
		}
	}

	if q.h.Len() < q.capacity {
		q.push(trip)
		q.notEmpty.Signal()
		return EnqueueResult{
			Status: PushStatusQueued,
		}
	}

	mostRecentIndex := q.getMostRecentIndex()
	mostRecent := *q.h[mostRecentIndex]

	if !higherPriority(trip, mostRecent.trip) {
		return EnqueueResult{
			Status: PushStatusDropped,
		}
	}

	removed := standart_heap.Remove(&q.h, mostRecentIndex).(*queueItem)
	delete(q.getByKey, removed.trip.TripID)

	q.push(trip)

	return EnqueueResult{
		Status:  PushStatusQueued,
		Removed: &removed.trip,
	}
}

func (q *PriorityQueue) Take() (SelectedTrip, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for len(q.h) == 0 && !q.closed {
		q.notEmpty.Wait()
	}

	if q.closed && len(q.h) == 0 {
		return SelectedTrip{}, false
	}

	pop := standart_heap.Pop(&q.h).(*queueItem)

	return pop.trip, true
}

func (q *PriorityQueue) Complete(tripID uuid.UUID) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	item, ok := q.getByKey[tripID]
	if !ok || item.index >= 0 {
		return
	}
	delete(q.getByKey, tripID)
}

func (q *PriorityQueue) Cancel(tripID uuid.UUID) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	item, ok := q.getByKey[tripID]
	if !ok || item.index < 0 {
		return
	}

	removed := standart_heap.Remove(&q.h, item.index).(*queueItem)

	delete(q.getByKey, removed.trip.TripID)
}

func (q *PriorityQueue) Close() {
	q.mutex.Lock()
	q.closed = true
	q.notEmpty.Broadcast()
	q.mutex.Unlock()
}

func (h *heap) Push(x any) {
	item := x.(*queueItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *heap) Pop() any {
	slice := *h
	lastIdx := len(slice) - 1
	item := slice[lastIdx]

	slice[lastIdx] = nil
	item.index = -1
	*h = slice[:lastIdx]
	return item
}

func (h heap) Len() int {
	return len(h)
}

func (h heap) Less(i, j int) bool {
	return higherPriority(h[i].trip, h[j].trip)
}

func (h heap) Swap(i, j int) {
	tmp := h[i]
	h[i] = h[j]
	h[j] = tmp

	h[i].index = i
	h[j].index = j
}

func higherPriority(a, b SelectedTrip) bool {
	return a.DontTrackedAfter.Before(b.DontTrackedAfter)
}

var _ standart_heap.Interface = (*heap)(nil)

func (q *PriorityQueue) push(trip SelectedTrip) {
	item := &queueItem{
		trip: trip,
	}
	standart_heap.Push(&q.h, item)
	q.getByKey[item.trip.TripID] = item
}

func (q *PriorityQueue) getMostRecentIndex() int {
	if len(q.h) == 0 {
		return -1
	}
	result := 0

	for i := 1; i < len(q.h); i++ {
		if higherPriority(q.h[result].trip, q.h[i].trip) {
			result = i
		}
	}

	return result
}
