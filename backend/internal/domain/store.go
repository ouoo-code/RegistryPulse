package domain

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	Categories() []Category
	Category(slug string) (Category, error)
	Sources() []Source
	Source(id string) (Source, error)
	History(sourceID string, limit int) []ProbeResult
	UpsertSource(SourceInput, string) (Source, error)
	DeleteSource(id string) error
}

// AgentRegistry is the persistence boundary for probe nodes and leased tasks.
// The in-memory implementation remains useful for unit tests and no-DB mode;
// production API processes use the PostgreSQL implementation.
type AgentRegistry interface {
	Register(AgentRegisterInput, string) ProbeNode
	Authenticate(string) (ProbeNode, bool)
	Heartbeat(string, AgentHeartbeatInput) (ProbeNode, error)
	Poll(string, int, time.Duration) []ProbeTask
	StartTask(string, string) (ProbeTask, error)
	CompleteTask(string, string, AgentResultInput) (ProbeTask, error)
	FailTask(string, string, string) (ProbeTask, error)
	Enqueue(string, string, map[string]any) ProbeTask
	Tasks(int) []ProbeTask
	Nodes() []ProbeNode
	Remove(string) error
	CancelTask(string) (ProbeTask, error)
	RetryTask(string) (ProbeTask, error)
	ClearTasks() (int64, error)
}

type MemoryStore struct {
	mu         sync.RWMutex
	categories []Category
	sources    map[string]Source
	history    map[string][]ProbeResult
}

func NewMemoryStore() *MemoryStore {
	now := time.Now().UTC()
	categories := []Category{
		{ID: "dockerhub", Slug: "dockerhub", Name: "Docker Hub", Description: "Docker 官方镜像仓库", Enabled: true, CreatedAt: now},
		{ID: "ghcr", Slug: "ghcr", Name: "GitHub Container Registry", Description: "GitHub 的 OCI 镜像仓库", Enabled: true, CreatedAt: now},
		{ID: "quay", Slug: "quay", Name: "Quay", Description: "Red Hat Quay 镜像仓库", Enabled: true, CreatedAt: now},
		{ID: "mcr", Slug: "mcr", Name: "Microsoft Container Registry", Description: "Microsoft 容器镜像仓库", Enabled: true, CreatedAt: now},
		{ID: "k8s", Slug: "k8s", Name: "Kubernetes Registry", Description: "Kubernetes 相关镜像仓库", Enabled: true, SortOrder: 50, CreatedAt: now},
		{ID: "gcr", Slug: "gcr", Name: "Google Container Registry", Description: "Google 容器镜像仓库", Enabled: true, SortOrder: 60, CreatedAt: now},
		{ID: "elastic", Slug: "elastic", Name: "Elastic Container Registry", Description: "Elastic 官方镜像仓库", Enabled: true, SortOrder: 70, CreatedAt: now},
		{ID: "nvcr", Slug: "nvcr", Name: "NVIDIA Container Registry", Description: "NVIDIA 容器镜像仓库", Enabled: true, SortOrder: 80, CreatedAt: now},
		{ID: "custom", Slug: "custom", Name: "自定义 OCI Registry", Description: "自定义 OCI 镜像仓库", Enabled: true, SortOrder: 90, CreatedAt: now},
	}
	return &MemoryStore{categories: categories, sources: map[string]Source{
		"dockerhub-official": {ID: "dockerhub-official", CategoryID: "dockerhub", Name: "Docker Hub 官方", BaseURL: "https://registry-1.docker.io", DisplayURL: "https://registry-1.docker.io", Provider: "Docker", Region: "Global", Tags: []string{"official", "oci"}, Enabled: true, Status: "unknown", CreatedAt: now, UpdatedAt: now},
	}, history: make(map[string][]ProbeResult)}
}

func (s *MemoryStore) Categories() []Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]Category(nil), s.categories...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Name < out[j].Name
	})
	return out
}
func (s *MemoryStore) Category(slug string) (Category, error) {
	for _, c := range s.Categories() {
		if c.Slug == slug || c.ID == slug {
			return c, nil
		}
	}
	return Category{}, ErrNotFound
}
func (s *MemoryStore) Sources() []Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Source, 0, len(s.sources))
	for _, v := range s.sources {
		v.Tags = append([]string(nil), v.Tags...)
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out
}
func (s *MemoryStore) Source(id string) (Source, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.sources[id]
	if !ok {
		return Source{}, ErrNotFound
	}
	return v, nil
}
func (s *MemoryStore) History(id string, limit int) []ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	h := s.history[id]
	if len(h) > limit {
		h = h[len(h)-limit:]
	}
	return append([]ProbeResult(nil), h...)
}
func (s *MemoryStore) UpsertSource(in SourceInput, id string) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if id == "" {
		id = "source-" + now.Format("20060102150405.000000000")
	}
	old, exists := s.sources[id]
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if exists {
		old.CategoryID, old.Name, old.BaseURL, old.DisplayURL, old.Description, old.Provider, old.Country, old.Region, old.Operator, old.Tags, old.Enabled = in.CategoryID, in.Name, in.BaseURL, in.DisplayURL, in.Description, in.Provider, in.Country, in.Region, in.Operator, append([]string(nil), in.Tags...), enabled
		if old.DisplayURL == "" {
			old.DisplayURL = old.BaseURL
		}
		if in.IsOfficial != nil {
			old.IsOfficial = *in.IsOfficial
		}
		if in.IsCloudflare != nil {
			old.IsCloudflare = *in.IsCloudflare
		}
		if in.IsRecommended != nil {
			old.IsRecommended = *in.IsRecommended
		}
		if in.Priority != nil {
			old.Priority = *in.Priority
		}
		if in.SortOrder != nil {
			old.SortOrder = *in.SortOrder
		}
		if in.Maintenance != nil {
			old.Maintenance = *in.Maintenance
			if old.Maintenance {
				old.Status = "maintenance"
			} else if old.Status == "maintenance" {
				old.Status = "unknown"
			}
		}
		if in.ProbeMode != "" {
			old.ProbeMode = in.ProbeMode
		}
		old.UpdatedAt = now
		s.sources[id] = old
		return old, nil
	}
	maintenance := false
	if in.Maintenance != nil {
		maintenance = *in.Maintenance
	}
	v := Source{ID: id, CategoryID: in.CategoryID, Name: in.Name, BaseURL: in.BaseURL, DisplayURL: in.DisplayURL, Provider: in.Provider, Region: in.Region, Tags: append([]string(nil), in.Tags...), Enabled: enabled, Maintenance: maintenance, Status: "unknown", CreatedAt: now, UpdatedAt: now}
	if v.DisplayURL == "" {
		v.DisplayURL = v.BaseURL
	}
	v.Description, v.Country, v.Operator = in.Description, in.Country, in.Operator
	if in.IsOfficial != nil {
		v.IsOfficial = *in.IsOfficial
	}
	if in.IsCloudflare != nil {
		v.IsCloudflare = *in.IsCloudflare
	}
	if in.IsRecommended != nil {
		v.IsRecommended = *in.IsRecommended
	}
	if in.Priority != nil {
		v.Priority = *in.Priority
	}
	if in.SortOrder != nil {
		v.SortOrder = *in.SortOrder
	}
	v.TestRepository, v.TestTag, v.TestDigest = in.TestRepository, in.TestTag, in.TestDigest
	if v.TestRepository == "" {
		v.TestRepository = "library/alpine"
	}
	if v.TestTag == "" {
		v.TestTag = "latest"
	}
	v.RequestTimeout, v.DownloadTestBytes = 10, 2<<20
	if in.RequestTimeout != nil && *in.RequestTimeout >= 1 && *in.RequestTimeout <= 300 {
		v.RequestTimeout = *in.RequestTimeout
	}
	if in.DownloadTestBytes != nil && *in.DownloadTestBytes >= 0 && *in.DownloadTestBytes <= 64<<20 {
		v.DownloadTestBytes = *in.DownloadTestBytes
	}
	v.ProbeMode = in.ProbeMode
	if v.ProbeMode == "" {
		v.ProbeMode = "registry"
	}
	s.sources[id] = v
	return v, nil
}
func (s *MemoryStore) DeleteSource(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sources[id]; !ok {
		return ErrNotFound
	}
	delete(s.sources, id)
	delete(s.history, id)
	return nil
}

var (
	ErrAgentUnauthorized = errors.New("agent unauthorized")
	ErrTaskNotAssigned   = errors.New("task is not assigned to this agent")
	ErrTaskState         = errors.New("invalid task state")
)

// MemoryAgentRegistry provides the protocol/state boundary used by the HTTP
// agent API. It is intentionally independent from the source Store so a
// PostgreSQL/Redis implementation can replace it without changing handlers.
type MemoryAgentRegistry struct {
	mu       sync.Mutex
	sequence uint64
	nodes    map[string]ProbeNode
	tokens   map[[32]byte]string
	tasks    map[string]ProbeTask
}

func NewMemoryAgentRegistry() *MemoryAgentRegistry {
	return &MemoryAgentRegistry{nodes: make(map[string]ProbeNode), tokens: make(map[[32]byte]string), tasks: make(map[string]ProbeTask)}
}

func (r *MemoryAgentRegistry) nextID(prefix string) string {
	r.sequence++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), r.sequence)
}

func tokenDigest(token string) [32]byte { return sha256.Sum256([]byte(token)) }

func (r *MemoryAgentRegistry) Register(input AgentRegisterInput, token string) ProbeNode {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	node := ProbeNode{ID: r.nextID("probe"), Name: input.Name, Region: input.Region, Version: input.Version, Capabilities: append([]string(nil), input.Capabilities...), Status: "online", LastSeenAt: now, CreatedAt: now, UpdatedAt: now}
	r.nodes[node.ID] = node
	r.tokens[tokenDigest(token)] = node.ID
	return node
}

func (r *MemoryAgentRegistry) Authenticate(token string) (ProbeNode, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.tokens[tokenDigest(token)]
	if !ok {
		return ProbeNode{}, false
	}
	node, ok := r.nodes[id]
	return node, ok
}

func (r *MemoryAgentRegistry) Heartbeat(id string, input AgentHeartbeatInput) (ProbeNode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	node, ok := r.nodes[id]
	if !ok {
		return ProbeNode{}, ErrAgentUnauthorized
	}
	if input.Status != "" {
		node.Status = input.Status
	}
	if node.Status == "offline" {
		node.Status = "online"
	}
	if input.Version != "" {
		node.Version = input.Version
	}
	if input.Capabilities != nil {
		node.Capabilities = append([]string(nil), input.Capabilities...)
	}
	node.LastSeenAt, node.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	r.nodes[id] = node
	return node, nil
}

func (r *MemoryAgentRegistry) Poll(id string, limit int, lease time.Duration) []ProbeTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	now := time.Now().UTC()
	out := make([]ProbeTask, 0, limit)
	for taskID, task := range r.tasks {
		if len(out) >= limit {
			break
		}
		if task.Status == "leased" && task.LeaseUntil.Before(now) {
			task.Status, task.ProbeNodeID = "pending", ""
		}
		if task.Status != "pending" {
			r.tasks[taskID] = task
			continue
		}
		task.Status, task.ProbeNodeID, task.LeaseUntil = "leased", id, now.Add(lease)
		task.UpdatedAt = now
		r.tasks[taskID] = task
		out = append(out, task)
	}
	return out
}

func (r *MemoryAgentRegistry) StartTask(agentID, taskID string) (ProbeTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ProbeTask{}, ErrNotFound
	}
	if task.ProbeNodeID != agentID || (task.Status != "leased" && task.Status != "running") {
		return ProbeTask{}, ErrTaskNotAssigned
	}
	task.Status, task.StartedAt, task.UpdatedAt = "running", time.Now().UTC(), time.Now().UTC()
	r.tasks[taskID] = task
	return task, nil
}

func (r *MemoryAgentRegistry) CompleteTask(agentID, taskID string, input AgentResultInput) (ProbeTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ProbeTask{}, ErrNotFound
	}
	if task.ProbeNodeID != agentID || (task.Status != "running" && task.Status != "leased") {
		return ProbeTask{}, ErrTaskNotAssigned
	}
	now := time.Now().UTC()
	result := &ProbeResult{ID: r.nextID("result"), SourceID: input.SourceID, ProbeNodeID: agentID, TaskID: taskID, Status: input.Status, DNSMS: input.DNSMS, TCPMS: input.TCPMS, TLSMS: input.TLSMS, RegistryMS: input.RegistryMS, ManifestMS: input.ManifestMS, Error: input.Error, CheckedAt: now}
	task.Status, task.Result, task.FinishedAt, task.UpdatedAt = "completed", result, now, now
	r.tasks[taskID] = task
	return task, nil
}

func (r *MemoryAgentRegistry) FailTask(agentID, taskID, message string) (ProbeTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ProbeTask{}, ErrNotFound
	}
	if task.ProbeNodeID != agentID || (task.Status != "running" && task.Status != "leased") {
		return ProbeTask{}, ErrTaskNotAssigned
	}
	now := time.Now().UTC()
	task.Status, task.Error, task.FinishedAt, task.UpdatedAt = "failed", message, now, now
	r.tasks[taskID] = task
	return task, nil
}

// Enqueue is useful to a scheduler implementation and keeps task creation
// separate from the agent-facing protocol.
func (r *MemoryAgentRegistry) Enqueue(sourceID, taskType string, payload map[string]any) ProbeTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	task := ProbeTask{ID: r.nextID("task"), SourceID: sourceID, Type: taskType, Payload: payload, Status: "pending", CreatedAt: now, UpdatedAt: now}
	r.tasks[task.ID] = task
	return task
}

func (r *MemoryAgentRegistry) Tasks(limit int) []ProbeTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]ProbeTask, 0, limit)
	for _, task := range r.tasks {
		if len(out) >= limit {
			break
		}
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (r *MemoryAgentRegistry) Nodes() []ProbeNode {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ProbeNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *MemoryAgentRegistry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.nodes[id]; !ok {
		return ErrNotFound
	}
	delete(r.nodes, id)
	for token, nodeID := range r.tokens {
		if nodeID == id {
			delete(r.tokens, token)
		}
	}
	for taskID, task := range r.tasks {
		if task.ProbeNodeID == id && (task.Status == "leased" || task.Status == "running") {
			task.Status, task.ProbeNodeID = "pending", ""
			r.tasks[taskID] = task
		}
	}
	return nil
}

func (r *MemoryAgentRegistry) CancelTask(id string) (ProbeTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return ProbeTask{}, ErrNotFound
	}
	if t.Status == "completed" || t.Status == "failed" || t.Status == "cancelled" {
		return ProbeTask{}, ErrTaskState
	}
	t.Status = "cancelled"
	t.UpdatedAt = time.Now().UTC()
	r.tasks[id] = t
	return t, nil
}
func (r *MemoryAgentRegistry) RetryTask(id string) (ProbeTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return ProbeTask{}, ErrNotFound
	}
	if t.Status != "failed" && t.Status != "cancelled" {
		return ProbeTask{}, ErrTaskState
	}
	t.Status = "pending"
	t.ProbeNodeID = ""
	t.Error = ""
	t.FinishedAt = time.Time{}
	t.UpdatedAt = time.Now().UTC()
	r.tasks[id] = t
	return t, nil
}

func (r *MemoryAgentRegistry) ClearTasks() (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var deleted int64
	for id, task := range r.tasks {
		if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
			delete(r.tasks, id)
			deleted++
		}
	}
	return deleted, nil
}
