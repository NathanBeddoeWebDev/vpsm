package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/util"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"golang.org/x/sync/errgroup"
)

// ErrAborted is returned when a user cancels the interactive flow.
var ErrAborted = errors.New("server creation aborted by user")

type catalogData struct {
	locations   []domain.Location
	serverTypes []domain.ServerTypeSpec
	images      []domain.ImageSpec
	sshKeys     []domain.SSHKeySpec
}

// CreateServerForm runs an interactive wizard that collects server create options.
// It fetches all catalog data up front, then walks the user through selection.
// Server types are filtered client-side by the chosen location to prevent
// "unsupported location for server type" errors at creation time.
func CreateServerForm(provider domain.CatalogProvider, prefill domain.CreateServerOpts) (*domain.CreateServerOpts, error) {
	accessible := os.Getenv("ACCESSIBLE") != ""

	// Fetch all catalog data concurrently in a single spinner.
	var data catalogData
	fetchErr := spinner.New().
		Title("Fetching server options...").
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
		return nil, fmt.Errorf("no locations available")
	}
	if len(data.serverTypes) == 0 {
		return nil, fmt.Errorf("no server types available")
	}
	if len(data.images) == 0 {
		return nil, fmt.Errorf("no images available")
	}

	opts := prefill
	opts.SSHKeys = append([]string(nil), prefill.SSHKeys...)

	// --- Form 1: Name + Location ---

	locationOpts, locationLabels := buildLocationOptions(data.locations, opts.Location)

	nameField := huh.NewInput().
		Title("Server name").
		Value(&opts.Name).
		Validate(func(value string) error {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				return errors.New("name is required")
			}
			return util.ValidateServerName(trimmed)
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

	// --- Filter server types by selected location ---

	filteredTypes := filterServerTypesByLocation(data.serverTypes, opts.Location)
	if len(filteredTypes) == 0 {
		return nil, fmt.Errorf("no server types available for location %q", opts.Location)
	}

	// Clear prefilled server type if it's not available at this location.
	if opts.ServerType != "" && !hasServerType(filteredTypes, opts.ServerType) {
		opts.ServerType = ""
	}

	serverTypeOpts, serverTypeLabels := buildServerTypeOptions(filteredTypes, opts.ServerType)

	// Build a name->spec lookup for architecture-based image filtering.
	typeByName := make(map[string]domain.ServerTypeSpec, len(data.serverTypes))
	for _, st := range data.serverTypes {
		typeByName[st.Name] = st
	}

	var imageLabels map[string]string
	imageOptsFunc := func() []huh.Option[string] {
		arch := ""
		if st, ok := typeByName[opts.ServerType]; ok {
			arch = st.Architecture
		}
		options, labels := buildImageOptions(data.images, arch, opts.Image)
		imageLabels = labels
		return options
	}
	_ = imageOptsFunc() // prime imageLabels for summary

	sshKeyOpts, sshKeyLabels := buildSSHKeyOptions(data.sshKeys, opts.SSHKeys)

	// --- Form 2: Server Type + Image + SSH Keys + Confirm ---

	serverTypeField := huh.NewSelect[string]().
		Title("Server type").
		Options(serverTypeOpts...).
		Value(&opts.ServerType).
		Height(selectHeight(len(serverTypeOpts), 12)).
		Validate(huh.ValidateNotEmpty())

	imageField := huh.NewSelect[string]().
		Title("Image").
		OptionsFunc(imageOptsFunc, &opts.ServerType).
		Value(&opts.Image).
		Height(12).
		Validate(huh.ValidateNotEmpty())

	var sshKeyGroup *huh.Group
	if len(sshKeyOpts) == 0 {
		sshKeyGroup = huh.NewGroup(
			huh.NewNote().
				Title("SSH keys").
				Description("No SSH keys found for this account."),
		)
	} else {
		sshKeyGroup = huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("SSH keys").
				Options(sshKeyOpts...).
				Value(&opts.SSHKeys).
				Height(10),
		)
	}

	confirm := false
	summaryNote := huh.NewNote().
		Title("Summary").
		DescriptionFunc(func() string {
			s := opts
			s.Name = strings.TrimSpace(s.Name)
			return buildSummary(s, locationLabels, serverTypeLabels, imageLabels, sshKeyLabels)
		}, &opts)

	confirmField := huh.NewConfirm().
		Title("Create this server?").
		Value(&confirm)

	if err := runForm(accessible,
		huh.NewGroup(serverTypeField),
		huh.NewGroup(imageField),
		sshKeyGroup,
		huh.NewGroup(summaryNote, confirmField),
	); err != nil {
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

// runForm creates and runs a huh.Form, translating ErrUserAborted to ErrAborted.
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

// fetchCatalog fetches locations, server types, images, and SSH keys concurrently.
func fetchCatalog(ctx context.Context, provider domain.CatalogProvider) (catalogData, error) {
	var data catalogData
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		data.locations, err = provider.ListLocations()
		if err != nil {
			return fmt.Errorf("failed to list locations: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		data.serverTypes, err = provider.ListServerTypes()
		if err != nil {
			return fmt.Errorf("failed to list server types: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		data.images, err = provider.ListImages()
		if err != nil {
			return fmt.Errorf("failed to list images: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		data.sshKeys, err = provider.ListSSHKeys()
		if err != nil {
			return fmt.Errorf("failed to list SSH keys: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return catalogData{}, err
	}
	return data, nil
}

// --- Filtering ---

// filterServerTypesByLocation returns only server types available at the given
// location. If location is empty (auto), all types are returned.
func filterServerTypesByLocation(serverTypes []domain.ServerTypeSpec, location string) []domain.ServerTypeSpec {
	if location == "" {
		return serverTypes
	}

	filtered := make([]domain.ServerTypeSpec, 0, len(serverTypes))
	for _, st := range serverTypes {
		if hasLocation(st.Locations, location) {
			filtered = append(filtered, st)
		}
	}
	return filtered
}

func hasLocation(locations []string, target string) bool {
	for _, loc := range locations {
		if strings.EqualFold(loc, target) {
			return true
		}
	}
	return false
}

func hasServerType(serverTypes []domain.ServerTypeSpec, name string) bool {
	for _, st := range serverTypes {
		if strings.EqualFold(st.Name, name) || st.ID == name {
			return true
		}
	}
	return false
}

// --- Option builders ---

func buildLocationOptions(locations []domain.Location, selected string) ([]huh.Option[string], map[string]string) {
	options := make([]huh.Option[string], 0, len(locations))
	labels := make(map[string]string, len(locations))

	for _, loc := range locations {
		value := valueOrID(loc.Name, loc.ID)
		label := locationLabel(loc)
		options = append(options, huh.NewOption(label, value))
		labels[value] = label
	}

	if selected != "" {
		options = ensureOption(options, labels, selected, "Custom: "+selected)
	}

	return options, labels
}

func buildServerTypeOptions(serverTypes []domain.ServerTypeSpec, selected string) ([]huh.Option[string], map[string]string) {
	options := make([]huh.Option[string], 0, len(serverTypes))
	labels := make(map[string]string, len(serverTypes))

	for _, st := range serverTypes {
		value := valueOrID(st.Name, st.ID)
		label := serverTypeLabel(st)
		options = append(options, huh.NewOption(label, value))
		labels[value] = label
	}

	if selected != "" {
		options = ensureOption(options, labels, selected, "Custom: "+selected)
	}

	return options, labels
}

func buildImageOptions(images []domain.ImageSpec, arch string, selected string) ([]huh.Option[string], map[string]string) {
	filtered := filterImages(images, arch)
	options := make([]huh.Option[string], 0, len(filtered))
	labels := make(map[string]string, len(filtered))

	for _, img := range filtered {
		value := valueOrID(img.Name, img.ID)
		label := imageLabel(img)
		options = append(options, huh.NewOption(label, value))
		labels[value] = label
	}

	if selected != "" {
		options = ensureOption(options, labels, selected, "Custom: "+selected)
	}

	return options, labels
}

func buildSSHKeyOptions(keys []domain.SSHKeySpec, selected []string) ([]huh.Option[string], map[string]string) {
	options := make([]huh.Option[string], 0, len(keys))
	labels := make(map[string]string, len(keys))

	for _, key := range keys {
		value := valueOrID(key.Name, key.ID)
		label := sshKeyLabel(key)
		options = append(options, huh.NewOption(label, value))
		labels[value] = label
	}

	for _, value := range selected {
		if value != "" {
			options = ensureOption(options, labels, value, "Custom: "+value)
		}
	}

	return options, labels
}

func ensureOption(options []huh.Option[string], labels map[string]string, value string, label string) []huh.Option[string] {
	if value == "" {
		return options
	}
	if _, ok := labels[value]; ok {
		return options
	}
	options = append(options, huh.NewOption(label, value))
	labels[value] = label
	return options
}

// --- Summary ---

func buildSummary(opts domain.CreateServerOpts, locationLabels, serverTypeLabels, imageLabels, sshKeyLabels map[string]string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Name: %s\n", opts.Name)
	fmt.Fprintf(&b, "Location: %s\n", labelFor(locationLabels, opts.Location, "Auto (provider default)"))
	fmt.Fprintf(&b, "Server type: %s\n", labelFor(serverTypeLabels, opts.ServerType, "Not selected"))
	fmt.Fprintf(&b, "Image: %s\n", labelFor(imageLabels, opts.Image, "Not selected"))
	fmt.Fprintf(&b, "SSH keys: %s\n", formatList(opts.SSHKeys, sshKeyLabels, "None"))

	if labels := formatLabels(opts.Labels); labels != "" {
		fmt.Fprintf(&b, "Labels: %s\n", labels)
	}
	if opts.UserData != "" {
		fmt.Fprintf(&b, "User data: %d bytes\n", len(opts.UserData))
	}
	if opts.StartAfterCreate != nil {
		fmt.Fprintf(&b, "Start after create: %t\n", *opts.StartAfterCreate)
	}

	return strings.TrimSpace(b.String())
}

// --- Image filtering ---

func filterImages(images []domain.ImageSpec, arch string) []domain.ImageSpec {
	if len(images) == 0 {
		return nil
	}

	// Prefer system images over snapshots/backups.
	systemImages := make([]domain.ImageSpec, 0, len(images))
	for _, img := range images {
		if strings.EqualFold(img.Type, "system") {
			systemImages = append(systemImages, img)
		}
	}
	filtered := images
	if len(systemImages) > 0 {
		filtered = systemImages
	}

	if arch == "" {
		return filtered
	}

	// Filter by architecture, but fall back to all if nothing matches.
	archFiltered := make([]domain.ImageSpec, 0, len(filtered))
	for _, img := range filtered {
		if strings.EqualFold(img.Architecture, arch) {
			archFiltered = append(archFiltered, img)
		}
	}
	if len(archFiltered) > 0 {
		return archFiltered
	}
	return filtered
}

// --- Label helpers ---

func locationLabel(loc domain.Location) string {
	name := valueOrID(loc.Name, loc.ID)
	suffix := strings.TrimSpace(loc.City + ", " + loc.Country)
	if suffix == ", " || suffix == "" {
		return name
	}
	return name + " - " + suffix
}

func serverTypeLabel(st domain.ServerTypeSpec) string {
	name := valueOrID(st.Name, st.ID)
	memory := strconv.FormatFloat(st.Memory, 'f', -1, 64)
	label := fmt.Sprintf("%s - %d vCPU / %s GB / %d GB", name, st.Cores, memory, st.Disk)
	if st.PriceMonthly != "" {
		return label + " - " + st.PriceMonthly + "/mo"
	}
	if st.PriceHourly != "" {
		return label + " - " + st.PriceHourly + "/hr"
	}
	return label
}

func imageLabel(img domain.ImageSpec) string {
	name := valueOrID(img.Name, img.ID)
	label := name
	if img.Description != "" {
		label = name + " - " + img.Description
	}
	if img.Architecture != "" {
		label += " (" + img.Architecture + ")"
	}
	return label
}

func sshKeyLabel(key domain.SSHKeySpec) string {
	name := valueOrID(key.Name, key.ID)
	if key.Fingerprint == "" {
		return name
	}
	return name + " (" + key.Fingerprint + ")"
}

func valueOrID(name string, id string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	return strings.TrimSpace(id)
}

func labelFor(labels map[string]string, value string, emptyLabel string) string {
	if value == "" {
		return emptyLabel
	}
	if labels != nil {
		if label, ok := labels[value]; ok {
			return label
		}
	}
	return value
}

func formatList(values []string, labels map[string]string, emptyLabel string) string {
	if len(values) == 0 {
		return emptyLabel
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		if label, ok := labels[v]; ok {
			parts = append(parts, label)
		} else {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ", ")
}

func selectHeight(optionCount, max int) int {
	if optionCount < max {
		return optionCount
	}
	return max
}
