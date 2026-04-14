package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"kate/services/tasks/internal/cache"

	"github.com/redis/go-redis/v9"
)

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DueDate     string    `json:"due_date"`
	Done        bool      `json:"done"`
	CreatedAt   time.Time `json:"created_at"`
}

type TaskService struct {
	mu    sync.RWMutex
	tasks map[string]Task
	redis *redis.Client
}

func NewTaskService() *TaskService {
	return &TaskService{
		tasks: make(map[string]Task),
	}
}

func NewTaskServiceWithRedis(redisClient *redis.Client) *TaskService {
	return &TaskService{
		tasks: make(map[string]Task),
		redis: redisClient,
	}
}

func (s *TaskService) Create(task Task) Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := "t_" + time.Now().Format("150405.000")
	task.ID = id
	task.CreatedAt = time.Now()
	s.tasks[id] = task
	return task
}

func (s *TaskService) GetAll() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		list = append(list, t)
	}
	return list
}

// GetByID с cache
func (s *TaskService) GetByID(ctx context.Context, id string) (Task, bool) {
	key := cache.TaskByIDKey(id)

	//  из Redis
	if s.redis != nil {
		cached, err := s.redis.Get(ctx, key).Result()
		if err == nil {
			var t Task
			if err := json.Unmarshal([]byte(cached), &t); err == nil {
				log.Println("[CACHE HIT]", key)
				return t, true
			}
		} else if err != redis.Nil {
			log.Println("[CACHE ERROR]", err)
		} else {
			log.Println("[CACHE MISS]", key)
		}
	}

	// Из памяти
	s.mu.RLock()
	t, ok := s.tasks[id]
	s.mu.RUnlock()

	if !ok {
		return Task{}, false
	}

	// Сохраняем в Redis
	if s.redis != nil {
		bytes, err := json.Marshal(t)
		if err == nil {
			ttl := cache.TTLWithJitter(120*time.Second, 30*time.Second)
			if err := s.redis.Set(ctx, key, bytes, ttl).Err(); err != nil {
				log.Println("[CACHE SET ERROR]", err)
			} else {
				log.Println("[CACHE SET]", key, "TTL:", ttl)
			}
		}
	}

	return t, true
}

func (s *TaskService) Update(id string, updated Task) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return Task{}, false
	}
	if updated.Title != "" {
		t.Title = updated.Title
	}
	if updated.Description != "" {
		t.Description = updated.Description
	}
	if updated.DueDate != "" {
		t.DueDate = updated.DueDate
	}
	t.Done = updated.Done
	s.tasks[id] = t

	// Инвалидация кэша
	if s.redis != nil {
		key := cache.TaskByIDKey(id)
		if err := s.redis.Del(context.Background(), key).Err(); err != nil {
			log.Println("[CACHE DEL ERROR]", err)
		} else {
			log.Println("[CACHE INVALIDATED]", key)
		}
	}

	return t, true
}

func (s *TaskService) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.tasks[id]
	if ok {
		delete(s.tasks, id)
	}

	// Инвалидация кэша
	if s.redis != nil {
		key := cache.TaskByIDKey(id)
		if err := s.redis.Del(context.Background(), key).Err(); err != nil {
			log.Println("[CACHE DEL ERROR]", err)
		} else {
			log.Println("[CACHE INVALIDATED]", key)
		}
	}

	return ok
}
