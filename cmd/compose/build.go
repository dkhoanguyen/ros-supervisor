package compose

type buildOptions struct {
	quiet    bool
	pull     bool
	progress string
	args     []string
	noCache  bool
	memory   string
}

type Project struct {
	Name       string `yaml:"-" json:"-"`
	WorkingDir string `yaml:"-" json:"-"`
	// Services     Services          `json:"services"`
	// Networks     Networks          `yaml:",omitempty" json:"networks,omitempty"`
	// Volumes      Volumes           `yaml:",omitempty" json:"volumes,omitempty"`
	// Secrets      Secrets           `yaml:",omitempty" json:"secrets,omitempty"`
	// Configs      Configs           `yaml:",omitempty" json:"configs,omitempty"`
	// Extensions   Extensions        `yaml:",inline" json:"-"` // https://github.com/golang/go/issues/6213
	ComposeFiles []string          `yaml:"-" json:"-"`
	Environment  map[string]string `yaml:"-" json:"-"`

	// DisabledServices track services which have been disable as profile is not active
	// DisabledServices Services `yaml:"-" json:"-"`
}
