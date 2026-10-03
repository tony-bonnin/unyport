package xenctl

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DefaultTimeout = 5 * time.Second

var (
	domainNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	hostNameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,253}$`)
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type Client struct {
	Bin     string
	Timeout time.Duration
	Runner  Runner
}

type Domain struct {
	DomID  int     `json:"domid"`
	Name   string  `json:"name"`
	State  string  `json:"state"`
	VCPUs  int     `json:"vcpus"`
	MemMB  uint64  `json:"mem_mb"`
	CPUSec float64 `json:"cpu_sec"`
}

type ActionResult struct {
	OK      bool     `json:"ok"`
	Action  string   `json:"action"`
	Args    []string `json:"args"`
	Output  string   `json:"output,omitempty"`
	DryRun  bool     `json:"dry_run,omitempty"`
	Message string   `json:"message,omitempty"`
}

func NewClient() *Client {
	return &Client{Bin: "xl", Timeout: DefaultTimeout, Runner: ExecRunner{}}
}

func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	out, err := c.run(ctx, "list")
	if err != nil {
		return nil, err
	}
	return ParseList(out), nil
}

func (c *Client) Info(ctx context.Context) (map[string]string, error) {
	out, err := c.run(ctx, "info")
	if err != nil {
		return nil, err
	}
	info := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		info[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return info, nil
}

func (c *Client) Shutdown(ctx context.Context, domain string, dryRun bool) (ActionResult, error) {
	return c.domainAction(ctx, dryRun, "shutdown", domain)
}

func (c *Client) Reboot(ctx context.Context, domain string, dryRun bool) (ActionResult, error) {
	return c.domainAction(ctx, dryRun, "reboot", domain)
}

func (c *Client) Pause(ctx context.Context, domain string, dryRun bool) (ActionResult, error) {
	return c.domainAction(ctx, dryRun, "pause", domain)
}

func (c *Client) Unpause(ctx context.Context, domain string, dryRun bool) (ActionResult, error) {
	return c.domainAction(ctx, dryRun, "unpause", domain)
}

func (c *Client) Destroy(ctx context.Context, domain string, dryRun bool) (ActionResult, error) {
	return c.domainAction(ctx, dryRun, "destroy", domain)
}

func (c *Client) Create(ctx context.Context, configPath string, dryRun bool) (ActionResult, error) {
	if !validConfigPath(configPath) {
		return ActionResult{}, errors.New("invalid config path")
	}
	return c.action(ctx, dryRun, "create", configPath)
}

func (c *Client) Migrate(ctx context.Context, domain string, targetHost string, live bool, dryRun bool) (ActionResult, error) {
	if err := validateDomain(domain); err != nil {
		return ActionResult{}, err
	}
	if !hostNameRE.MatchString(targetHost) {
		return ActionResult{}, errors.New("invalid target host")
	}
	args := []string{"migrate"}
	if live {
		args = append(args, "-l")
	}
	args = append(args, domain, targetHost)
	return c.action(ctx, dryRun, args...)
}

func (c *Client) domainAction(ctx context.Context, dryRun bool, action string, domain string) (ActionResult, error) {
	if err := validateDomain(domain); err != nil {
		return ActionResult{}, err
	}
	if isControlDomain(domain) {
		return ActionResult{}, errors.New("refusing to operate on Domain-0")
	}
	return c.action(ctx, dryRun, action, domain)
}

func (c *Client) action(ctx context.Context, dryRun bool, args ...string) (ActionResult, error) {
	result := ActionResult{OK: true, Action: args[0], Args: append([]string(nil), args...), DryRun: dryRun}
	if dryRun {
		result.Message = "validated"
		return result, nil
	}
	out, err := c.run(ctx, args...)
	result.Output = strings.TrimSpace(out)
	if err != nil {
		result.OK = false
		return result, err
	}
	return result, nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runner := c.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	bin := c.Bin
	if bin == "" {
		bin = "xl"
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := runner.Run(ctx, bin, args...)
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("xl %s timed out", args[0])
	}
	if err != nil {
		return string(out), fmt.Errorf("xl %s failed: %w", args[0], err)
	}
	return string(out), nil
}

func ParseList(out string) []Domain {
	lines := strings.Split(out, "\n")
	doms := make([]Domain, 0, len(lines))
	for i, line := range lines {
		if i == 0 {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		domid, err := strconv.Atoi(f[1])
		if err != nil {
			continue
		}
		mem, _ := strconv.ParseUint(f[2], 10, 64)
		vcpus, _ := strconv.Atoi(f[3])
		cpuSec, _ := strconv.ParseFloat(f[len(f)-1], 64)
		doms = append(doms, Domain{
			DomID:  domid,
			Name:   f[0],
			MemMB:  mem,
			VCPUs:  vcpus,
			State:  strings.Trim(f[4], "-"),
			CPUSec: cpuSec,
		})
	}
	return doms
}

func validateDomain(domain string) error {
	if !domainNameRE.MatchString(domain) {
		return errors.New("invalid domain name")
	}
	return nil
}

func isControlDomain(domain string) bool {
	return domain == "0" || strings.EqualFold(domain, "Domain-0")
}

func validConfigPath(path string) bool {
	if path == "" || strings.ContainsAny(path, "\x00\r\n") {
		return false
	}
	if !strings.HasPrefix(path, "/etc/xen/") && !strings.HasPrefix(path, "/media/") {
		return false
	}
	return strings.HasSuffix(path, ".cfg")
}
