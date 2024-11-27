package utils

import (
	"gopkg.in/yaml.v3"
)

// N.B. Minimal definition with what we're using
type CloudConfig struct {
	ChangePasswords      *CloudPasswordConfig `yaml:"chpasswd,omitempty"`
	EnableSSHPaswordAuth *bool                `yaml:"ssh_pwauth,omitempty"`
	DisableRootUser      *bool                `yaml:"disable_root,omitempty"`
	Users                []CloudUserConfig    `yaml:"users,omitempty"`
}

// N.B. Minimal definition with what we're using
type CloudPasswordConfig struct {
	List   string `yaml:"list,omitempty"`
	Expire *bool  `yaml:"expire,omitempty"`
}

// N.B. Minimal definition with what we're using
type CloudUserConfig struct {
	Description          string   `yaml:"gecos,omitempty"`
	EnableSSHPaswordAuth *bool    `yaml:"ssh_pwauth,omitempty"`
	Groups               []string `yaml:"groups,omitempty"`
	LockPassword         *bool    `yaml:"lock_passwd,omitempty"`
	Name                 string   `yaml:"name,omitempty"`
	PlainTextPassword    string   `yaml:"plain_text_passwd,omitempty"`
	Shell                string   `yaml:"shell,omitempty"`
	SSHAuthorizedKeys    []string `yaml:"ssh_authorized_keys,omitempty"`
	Sudo                 []string `yaml:"sudo,omitempty"`
}

func MarshalCloudConfigToYAML(config *CloudConfig) ([]byte, error) {
	bytes, err := yaml.Marshal(config)
	if err != nil {
		return []byte{}, err
	}

	// Prepend the cloud-config header.
	return append([]byte("#cloud-config\n"), bytes...), nil
}
