package fleet

import (
	"context"
	"time"
)

// Store isolates fleet control state from CPA's general-purpose RESP key/value namespace.
type Store interface {
	FleetLoad(context.Context) ([]byte, bool, error)
	FleetSwap(context.Context, []byte, bool, []byte) (bool, error)
	CurrentDatabaseTime(context.Context) (time.Time, error)
}

type NodePolicy struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type Policy struct {
	Enabled         bool         `json:"enabled"`
	CatalogSync     bool         `json:"catalog_sync"`
	PatchUpdates    bool         `json:"patch_updates"`
	IntervalSeconds int          `json:"interval_seconds"`
	Nodes           []NodePolicy `json:"nodes"`
}

type Inventory struct {
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Platform string `json:"platform"`
	Catalog  bool   `json:"catalog"`
}

type NodeState struct {
	Inventory
	SeenAt      time.Time `json:"seen_at"`
	CatalogHash string    `json:"catalog_hash"`
	CatalogAt   time.Time `json:"catalog_at"`
	LastJob     string    `json:"last_job,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
}

type Release struct {
	Version       string `json:"version"`
	URL           string `json:"url"`
	ArchiveSHA256 string `json:"archive_sha256"`
	SHA256        string `json:"sha256"`
	Size          int64  `json:"size"`
}

type Job struct {
	ID          string                    `json:"id"`
	NodeID      string                    `json:"node_id"`
	Operation   string                    `json:"operation"`
	Token       string                    `json:"token"`
	ExpiresAt   time.Time                 `json:"expires_at"`
	Before      Inventory                 `json:"before"`
	Release     *Release                  `json:"release,omitempty"`
	CatalogHash string                    `json:"catalog_hash,omitempty"`
	Metadata    map[string]map[string]any `json:"metadata,omitempty"`
}

type Result struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Inventory Inventory `json:"inventory"`
}

type Rollout struct {
	Approved  bool                 `json:"approved"`
	Release   Release              `json:"release"`
	Completed map[string]Inventory `json:"completed"`
	Phase     string               `json:"phase"`
	Error     string               `json:"error,omitempty"`
	Rollback  []string             `json:"rollback,omitempty"`
}

type Event struct {
	Time      time.Time `json:"time"`
	NodeID    string    `json:"node_id,omitempty"`
	Operation string    `json:"operation"`
	Status    string    `json:"status"`
	Version   string    `json:"version,omitempty"`
}

type State struct {
	Schema     int                  `json:"schema"`
	Revision   int64                `json:"revision"`
	Policy     Policy               `json:"policy"`
	Nodes      map[string]NodeState `json:"nodes"`
	Active     *Job                 `json:"active,omitempty"`
	Rollout    *Rollout             `json:"rollout,omitempty"`
	Latest     *Release             `json:"latest,omitempty"`
	LastCheck  time.Time            `json:"last_check"`
	CheckLease time.Time            `json:"check_lease"`
	CheckError string               `json:"check_error,omitempty"`
	History    []Event              `json:"history"`
}

type Plan struct {
	Accepted      *Release `json:"accepted_release,omitempty"`
	VerifiedNodes []string `json:"verified_node_ids,omitempty"`
	Enabled       bool     `json:"enabled"`
	PollSeconds   int      `json:"poll_seconds"`
	Job           *Job     `json:"job,omitempty"`
	Status        string   `json:"status"`
}
