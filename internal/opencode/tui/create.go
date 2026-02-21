// Package tui provides an interactive wizard for creating opencode VPS instances.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	opencodedomain "nathanbeddoewebdev/vpsm/internal/opencode/domain"
	serverdomain "nathanbeddoewebdev/vpsm/internal/server/domain"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"golang.org/x/sync/errgroup"
)

// ErrAborted is returned when the user cancels the wizard.
var ErrAborted = errors.New("opencode creation aborted by user")

// catalogData holds the fetched provider catalog needed for the wizard.
type catalogData struct {
	locations   []serverdomain.Location
	serverTypes []serverdomain.ServerTypeSpec
	sshKeys     []serverdomain.SSHKeySpec
}

// RunCreateWizard runs the interactive opencode VPS creation wizard.
// It fetches catalog data, walks the user through each option, and returns
// the populated CreateOpenCodeOpts ready for provisioning.
func RunCreateWizard(provider serverdomain.CatalogProvider, prefill opencodedomain.CreateOpenCodeOpts) (*opencodedomain.CreateOpenCodeOpts, error) {
	accessible := os.Getenv("ACCESSIBLE") != ""

	var data catalogData
	fetchErr := spinner.New().
		Title("Fetching provider options...").
		Accessible(accessible).
		Output(os.Stderr).
		ActionWithErr(func(ctx context.Context) error {
			var err error
			data, err = fetchCatalog(ctx, provider)
			return err
		}).
		Run()
	if fetchErr != nil {
		if errors.Is(fetchErr, huh.ErrUserAborted) || errors.Is(fetchErr, context.Canceled) {
			return nil, ErrAborted
		}
		return nil, fetchErr
	}

	if len(data.locations) == 0 {
		return nil, fmt.Errorf("no locations available from provider")
	}
	if len(data.serverTypes) == 0 {
		return nil, fmt.Errorf("no server types available from provider")
	}

	opts := prefill
	opts.SSHKeys = append([]string(nil), prefill.SSHKeys...)
	if opts.ProxyType == "" {
		opts.ProxyType = opencodedomain.ProxyTypeCaddy
	}

	// ── Step 1: Name + Location ────────────────────────────────────────────────

	locationOpts, locationLabels := buildLocationOptions(data.locations, opts.Location)

	nameField := huh.NewInput().
		Title("Server name").
		Placeholder("my-opencode").
		Value(&opts.Name).
		Validate(func(v string) error {
			v = strings.TrimSpace(v)
			if v == "" {
				return errors.New("name is required")
			}
			return nil
		})

	locationField := huh.NewSelect[string]().
		Title("Location").
		Options(locationOpts...).
		Value(&opts.Location).
		Height(selectHeight(len(locationOpts), 10))

	if err := runForm(accessible,
		huh.NewGroup(nameField),
		huh.NewGroup(locationField),
	); err != nil {
		return nil, err
	}

	// ── Step 2: Server Type ───────────────────────────────────────────────────

	filteredTypes := filterServerTypesByLocation(data.serverTypes, opts.Location)
	if len(filteredTypes) == 0 {
		return nil, fmt.Errorf("no server types available for location %q", opts.Location)
	}
	if opts.ServerType != "" && !hasServerType(filteredTypes, opts.ServerType) {
		opts.ServerType = ""
	}

	serverTypeOpts, serverTypeLabels := buildServerTypeOptions(filteredTypes, opts.ServerType)

	serverTypeField := huh.NewSelect[string]().
		Title("Server type").
		Description("opencode works best with at least 2 vCPU / 4 GB RAM (e.g. cpx21)").
		Options(serverTypeOpts...).
		Value(&opts.ServerType).
		Height(selectHeight(len(serverTypeOpts), 12)).
		Validate(huh.ValidateNotEmpty())

	if err := runForm(accessible, huh.NewGroup(serverTypeField)); err != nil {
		return nil, err
	}

	// ── Step 3: SSH Keys ──────────────────────────────────────────────────────

	sshKeyOpts, sshKeyLabels := buildSSHKeyOptions(data.sshKeys, opts.SSHKeys)

	var sshKeyGroup *huh.Group
	if len(sshKeyOpts) == 0 {
		sshKeyGroup = huh.NewGroup(
			huh.NewNote().
				Title("SSH keys").
				Description("No SSH keys found for this account.\nYou can add one with: vpsm sshkey upload"),
		)
	} else {
		sshKeyGroup = huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("SSH keys").
				Description("Select the keys to inject into the server").
				Options(sshKeyOpts...).
				Value(&opts.SSHKeys).
				Height(selectHeight(len(sshKeyOpts), 10)),
		)
	}

	if err := runForm(accessible, sshKeyGroup); err != nil {
		return nil, err
	}

	// ── Step 4: Proxy + Tailscale + Domain ────────────────────────────────────

	proxyStr := string(opts.ProxyType)
	proxyField := huh.NewSelect[string]().
		Title("Reverse proxy").
		Description("Serves the web terminal and deployed projects").
		Options(
			huh.NewOption("Caddy  (automatic HTTPS, recommended)", string(opencodedomain.ProxyTypeCaddy)),
			huh.NewOption("Nginx  (classic, widely supported)", string(opencodedomain.ProxyTypeNginx)),
		).
		Value(&proxyStr)

	tailscaleField := huh.NewInput().
		Title("Tailscale auth key (optional)").
		Description("Enables VPN access and MagicDNS so you can reach the server\nby hostname across the internet (e.g. http://my-opencode/terminal).\nLeave blank to skip.").
		Placeholder("tskey-auth-...").
		Value(&opts.TailscaleKey)

	domainField := huh.NewInput().
		Title("Public domain (optional)").
		Description("If set, Caddy requests a Let's Encrypt TLS certificate.\nOnly Caddy supports this option; the field is ignored for Nginx.\nExample: dev.example.com").
		Placeholder("dev.example.com").
		Value(&opts.Domain)

	if err := runForm(accessible,
		huh.NewGroup(proxyField),
		huh.NewGroup(tailscaleField),
		huh.NewGroup(domainField),
	); err != nil {
		return nil, err
	}

	opts.ProxyType = opencodedomain.ProxyType(proxyStr)

	// ── Step 5: Summary + Confirm ─────────────────────────────────────────────

	confirm := false
	summaryNote := huh.NewNote().
		Title("opencode VPS summary").
		DescriptionFunc(func() string {
			return buildSummary(opts, locationLabels, serverTypeLabels, sshKeyLabels)
		}, &opts)

	confirmField := huh.NewConfirm().
		Title("Provision this server?").
		Value(&confirm)

	if err := runForm(accessible, huh.NewGroup(summaryNote, confirmField)); err != nil {
		return nil, err
	}

	if !confirm {
		return nil, ErrAborted
	}

	opts.Name = strings.TrimSpace(opts.Name)
	if len(opts.SSHKeys) == 0 {
		opts.SSHKeys = nil
	}

	return &opts, nil
}

// ── Catalog fetch ─────────────────────────────────────────────────────────────

func fetchCatalog(ctx context.Context, provider serverdomain.CatalogProvider) (catalogData, error) {
	var data catalogData
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		data.locations, err = provider.ListLocations(gctx)
		if err != nil {
			return fmt.Errorf("list locations: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		data.serverTypes, err = provider.ListServerTypes(gctx)
		if err != nil {
			return fmt.Errorf("list server types: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		data.sshKeys, err = provider.ListSSHKeys(gctx)
		if err != nil {
			return fmt.Errorf("list SSH keys: %w", err)
		}
		return nil
	})

	return data, g.Wait()
}

// ── Option builders ───────────────────────────────────────────────────────────

func buildLocationOptions(locations []serverdomain.Location, selected string) ([]huh.Option[string], map[string]string) {
	sort.Slice(locations, func(i, j int) bool { return locations[i].Name < locations[j].Name })
	opts := make([]huh.Option[string], 0, len(locations))
	labels := make(map[string]string, len(locations))
	for _, loc := range locations {
		v := nameOrID(loc.Name, loc.ID)
		label := locationLabel(loc)
		opts = append(opts, huh.NewOption(label, v))
		labels[v] = label
	}
	if selected != "" {
		opts = ensureOption(opts, labels, selected)
	}
	return opts, labels
}

func buildServerTypeOptions(types []serverdomain.ServerTypeSpec, selected string) ([]huh.Option[string], map[string]string) {
	opts := make([]huh.Option[string], 0, len(types))
	labels := make(map[string]string, len(types))
	for _, st := range types {
		v := nameOrID(st.Name, st.ID)
		label := serverTypeLabel(st)
		opts = append(opts, huh.NewOption(label, v))
		labels[v] = label
	}
	if selected != "" {
		opts = ensureOption(opts, labels, selected)
	}
	return opts, labels
}

func buildSSHKeyOptions(keys []serverdomain.SSHKeySpec, selected []string) ([]huh.Option[string], map[string]string) {
	opts := make([]huh.Option[string], 0, len(keys))
	labels := make(map[string]string, len(keys))
	for _, key := range keys {
		v := nameOrID(key.Name, key.ID)
		label := sshKeyLabel(key)
		opts = append(opts, huh.NewOption(label, v))
		labels[v] = label
	}
	for _, v := range selected {
		opts = ensureOption(opts, labels, v)
	}
	return opts, labels
}

func ensureOption(opts []huh.Option[string], labels map[string]string, value string) []huh.Option[string] {
	if value == "" {
		return opts
	}
	if _, ok := labels[value]; ok {
		return opts
	}
	label := "Custom: " + value
	opts = append(opts, huh.NewOption(label, value))
	labels[value] = label
	return opts
}

// ── Filtering ─────────────────────────────────────────────────────────────────

func filterServerTypesByLocation(types []serverdomain.ServerTypeSpec, location string) []serverdomain.ServerTypeSpec {
	if location == "" {
		return types
	}
	filtered := make([]serverdomain.ServerTypeSpec, 0, len(types))
	for _, st := range types {
		for _, loc := range st.Locations {
			if strings.EqualFold(loc, location) {
				filtered = append(filtered, st)
				break
			}
		}
	}
	return filtered
}

func hasServerType(types []serverdomain.ServerTypeSpec, name string) bool {
	for _, st := range types {
		if strings.EqualFold(st.Name, name) || st.ID == name {
			return true
		}
	}
	return false
}

// ── Summary ───────────────────────────────────────────────────────────────────

func buildSummary(
	opts opencodedomain.CreateOpenCodeOpts,
	locationLabels, serverTypeLabels, sshKeyLabels map[string]string,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:         %s\n", opts.Name)
	fmt.Fprintf(&b, "Location:     %s\n", labelFor(locationLabels, opts.Location, "Auto"))
	fmt.Fprintf(&b, "Server type:  %s\n", labelFor(serverTypeLabels, opts.ServerType, "Not selected"))
	fmt.Fprintf(&b, "Image:        %s\n", opts.Image)

	if len(opts.SSHKeys) == 0 {
		fmt.Fprintf(&b, "SSH keys:     None\n")
	} else {
		names := make([]string, 0, len(opts.SSHKeys))
		for _, k := range opts.SSHKeys {
			names = append(names, labelFor(sshKeyLabels, k, k))
		}
		fmt.Fprintf(&b, "SSH keys:     %s\n", strings.Join(names, ", "))
	}

	fmt.Fprintf(&b, "Proxy:        %s\n", opts.ProxyType)

	if opts.TailscaleKey != "" {
		fmt.Fprintf(&b, "Tailscale:    enabled\n")
	}
	if opts.Domain != "" {
		fmt.Fprintf(&b, "Domain:       %s\n", opts.Domain)
	}

	fmt.Fprintf(&b, "\nSetup notes:\n")
	fmt.Fprintf(&b, "  - opencode, Node.js, and ttyd will be installed via cloud-init\n")
	fmt.Fprintf(&b, "  - SSH password auth and root login are disabled\n")
	fmt.Fprintf(&b, "  - UFW firewall + fail2ban are enabled\n")
	fmt.Fprintf(&b, "  - Avahi advertises the terminal via mDNS (_http._tcp)\n")
	if opts.TailscaleKey != "" {
		fmt.Fprintf(&b, "  - The server joins your tailnet as %q\n", opts.Name)
	}

	return strings.TrimRight(b.String(), "\n")
}

// ── Label helpers ─────────────────────────────────────────────────────────────

func locationLabel(loc serverdomain.Location) string {
	name := nameOrID(loc.Name, loc.ID)
	suffix := strings.TrimSpace(loc.City + ", " + loc.Country)
	if suffix == ", " || suffix == "" {
		return name
	}
	return name + "  —  " + suffix
}

func serverTypeLabel(st serverdomain.ServerTypeSpec) string {
	name := nameOrID(st.Name, st.ID)
	label := fmt.Sprintf("%s  —  %d vCPU / %.0f GB RAM / %d GB disk",
		name, st.Cores, st.Memory, st.Disk)
	if st.PriceMonthly != "" {
		return label + "  (" + st.PriceMonthly + "/mo)"
	}
	return label
}

func sshKeyLabel(key serverdomain.SSHKeySpec) string {
	name := nameOrID(key.Name, key.ID)
	if key.Fingerprint == "" {
		return name
	}
	return fmt.Sprintf("%s  [%s]", name, key.Fingerprint)
}

func nameOrID(name, id string) string {
	if name != "" {
		return name
	}
	return id
}

func labelFor(labels map[string]string, key, fallback string) string {
	if l, ok := labels[key]; ok {
		return l
	}
	if key != "" {
		return key
	}
	return fallback
}

// ── Form runner ───────────────────────────────────────────────────────────────

func runForm(accessible bool, groups ...*huh.Group) error {
	err := huh.NewForm(groups...).WithAccessible(accessible).Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrAborted
		}
		return err
	}
	return nil
}

func selectHeight(n, max int) int {
	if n < max {
		return n + 2
	}
	return max
}
