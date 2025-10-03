package flake

type FlakeLock struct {
	Nodes   map[string]Node `json:"nodes"`
	Root    string          `json:"root"`
	Version int             `json:"version"`
}

type Node struct {
	Locked   *Locked        `json:"locked,omitempty"`
	Original *Original      `json:"original,omitempty"`
	Inputs   map[string]any `json:"inputs,omitempty"`
}

type Locked struct {
	LastModified int64  `json:"lastModified,omitempty"`
	NarHash      string `json:"narHash,omitempty"`
	Owner        string `json:"owner,omitempty"`
	Repo         string `json:"repo,omitempty"`
	Rev          string `json:"rev,omitempty"`
	Type         string `json:"type,omitempty"`
	Host         string `json:"host,omitempty"`
	URL          string `json:"url,omitempty"`
	Path         string `json:"path,omitempty"`
}

type Original struct {
	Owner string `json:"owner,omitempty"`
	Ref   string `json:"ref,omitempty"`
	Repo  string `json:"repo,omitempty"`
	Type  string `json:"type,omitempty"`
}

type Relations struct {
	Deps        map[string][]string
	ReverseDeps map[string][]string
}

type Input struct {
	Type  string
	Owner string
	Repo  string
	Host  string
	URL   string
	Path  string
}
