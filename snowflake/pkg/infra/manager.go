package infra

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-logr/logr"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"antrea.io/theia/snowflake/database"
	sf "antrea.io/theia/snowflake/pkg/snowflake"
)

type pulumiPlugin struct {
	name    string
	version string
}

func createTemporaryWorkdir() (string, error) {
	return os.MkdirTemp("", "antrea-pulumi")
}

func deleteTemporaryWorkdir(d string) {
	os.RemoveAll(d)
}

func writeMigrationsToDisk(fsys fs.FS, migrationsPath string, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	entries, err := fs.ReadDir(fsys, migrationsPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := func() error {
			in, err := fsys.Open(filepath.Join(migrationsPath, e.Name()))
			if err != nil {
				return err
			}
			defer in.Close()

			out, err := os.Create(filepath.Join(dest, e.Name()))
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, in)
			if err != nil {
				return err
			}
			return out.Close()
		}(); err != nil {
			return err
		}
	}
	return nil
}

func downloadAndUntar(ctx context.Context, logger logr.Logger, url string, dir string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, hdr.Name)
		logger.V(4).Info("Untarring", "path", hdr.Name)
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if err := func() error {
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

func installPulumiCLI(ctx context.Context, logger logr.Logger, dir string) error {
	logger.Info("Downloading and installing Pulumi", "version", pulumiVersion)
	cachedVersion, err := os.ReadFile(filepath.Join(dir, ".pulumi-version"))
	if err == nil && string(cachedVersion) == pulumiVersion {
		logger.Info("Pulumi CLI is already up-to-date")
		return nil
	}

	operatingSystem := runtime.GOOS
	arch := runtime.GOARCH
	target := operatingSystem
	if arch == "amd64" {
		target += "-x64"
	} else if arch == "arm64" {
		target += "-arm64"
	} else {
		return fmt.Errorf("arch not supported: %s", arch)
	}
	supportedTargets := map[string]bool{
		"darwin-arm64": true,
		"darwin-x64":   true,
		"linux-arm64":  true,
		"linux-x64":    true,
		"windows-x64":  true,
	}
	if _, ok := supportedTargets[target]; !ok {
		return fmt.Errorf("OS / arch combination is not supported: %s / %s", operatingSystem, arch)
	}
	url := fmt.Sprintf("https://github.com/pulumi/pulumi/releases/download/%s/pulumi-%s-%s.tar.gz", pulumiVersion, pulumiVersion, target)
	if err := os.MkdirAll(filepath.Join(dir, "pulumi"), 0755); err != nil {
		return err
	}
	if err := downloadAndUntar(ctx, logger, url, dir); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, ".pulumi-version"), []byte(pulumiVersion), 0660); err != nil {
		logger.Error(err, "Error when writing pulumi version to cache file")
	}
	logger.Info("Installed Pulumi")
	return nil
}

func installMigrateSnowflakeCLI(ctx context.Context, logger logr.Logger, dir string) error {
	logger.Info("Downloading and installing Migrate Snowflake", "version", migrateSnowflakeVersion)
	cachedVersion, err := os.ReadFile(filepath.Join(dir, ".migrate-sf-version"))
	if err == nil && string(cachedVersion) == migrateSnowflakeVersion {
		logger.Info("Migrate Snowflake CLI is already up-to-date")
		return nil
	}

	operatingSystem := runtime.GOOS
	arch := runtime.GOARCH
	target := fmt.Sprintf("%s_%s", operatingSystem, arch)
	supportedTargets := map[string]bool{
		"darwin_arm64":  true,
		"darwin_amd64":  true,
		"linux_arm64":   true,
		"linux_amd64":   true,
		"windows_amd64": true,
	}
	if _, ok := supportedTargets[target]; !ok {
		return fmt.Errorf("OS / arch combination is not supported: %s / %s", operatingSystem, arch)
	}
	url := fmt.Sprintf("https://github.com/antoninbas/migrate-snowflake/releases/download/%s/migrate-snowflake_%s_%s.tar.gz", migrateSnowflakeVersion, migrateSnowflakeVersion, target)
	if err := downloadAndUntar(ctx, logger, url, dir); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, ".migrate-sf-version"), []byte(migrateSnowflakeVersion), 0660); err != nil {
		logger.Error(err, "Error when writing Migrate Snowflake version to cache file")
	}
	logger.Info("Installed Migrate Snowflake")
	return nil
}

type Manager struct {
	logger             logr.Logger
	stackName          string
	stateBackendURL    string
	secretsProviderURL string
	region             string
	warehouseName      string
	workdir            string
	verbose            bool
}

func NewManager(
	logger logr.Logger,
	stackName string,
	stateBackendURL string,
	secretsProviderURL string,
	region string,
	warehouseName string,
	workdir string,
	verbose bool, // output Pulumi progress to stdout
) *Manager {
	return &Manager{
		logger:             logger.WithValues("project", projectName, "stack", stackName),
		stackName:          stackName,
		stateBackendURL:    stateBackendURL,
		secretsProviderURL: secretsProviderURL,
		region:             region,
		warehouseName:      warehouseName,
		workdir:            workdir,
		verbose:            verbose,
	}
}

func (m *Manager) setup(
	ctx context.Context,
	stackName string,
	workdir string,
	requiredPlugins []pulumiPlugin,
	declareFunc func(ctx *pulumi.Context) error,
) (auto.Stack, error) {
	logger := m.logger
	logger.Info("Creating stack")
	secretsProvider := m.secretsProviderURL
	passphrase := os.Getenv("PULUMI_CONFIG_PASSPHRASE")
	// If KMS is not used to encrypt secrets, setting
	// PULUMI_CONFIG_PASSPHRASE is not useful. If neither is set, secrets
	// will be in "plain text" in the state backend (S3 bucket).
	if secretsProvider == "" && passphrase == "" {
		logger.Info("No secrets provider configured and PULUMI_CONFIG_PASSPHRASE env variable empty: secrets will be stored in plain text in backend")
	}
	opts := []auto.LocalWorkspaceOption{
		auto.WorkDir(workdir),
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			Main:    workdir,
			Backend: &workspace.ProjectBackend{
				URL: m.stateBackendURL,
			},
		}),
		auto.EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": os.Getenv("PULUMI_CONFIG_PASSPHRASE"),
		}),
	}
	if secretsProvider != "" {
		opts = append(
			opts,
			auto.SecretsProvider(secretsProvider),
			auto.Stacks(map[string]workspace.ProjectStack{
				stackName: {
					SecretsProvider: secretsProvider,
				},
			}),
		)
	}
	s, err := auto.UpsertStackInlineSource(
		ctx,
		fmt.Sprintf("%s.%s", projectName, stackName),
		projectName,
		declareFunc,
		opts...,
	)
	if err != nil {
		return s, err
	}
	logger.Info("Created stack")
	w := s.Workspace()
	for _, plugin := range requiredPlugins {
		logger.Info("Installing Pulumi plugin", "plugin", plugin.name, "version", plugin.version)
		if err := w.InstallPlugin(ctx, plugin.name, plugin.version); err != nil {
			return s, fmt.Errorf("failed to install Pulumi plugin %s: %w", plugin.name, err)
		}
		logger.Info("Installed Pulumi plugin", "plugin", plugin.name)
	}
	// set stack configuration specifying the AWS region to deploy
	s.SetConfig(ctx, "aws:region", auto.ConfigValue{Value: m.region})
	logger.Info("Refreshing stack")
	_, err = s.Refresh(ctx)
	if err != nil {
		return s, err
	}
	logger.Info("Refreshed stack")
	return s, nil
}

type Result struct {
	Region            string
	BucketName        string
	BucketFlowsFolder string
	DatabaseName      string
	SchemaName        string
	FlowsTableName    string
	SNSTopicARN       string
	SQSQueueARN       string
}

func (m *Manager) run(ctx context.Context, destroy bool) (*Result, error) {
	logger := m.logger
	workdir := m.workdir
	if workdir == "" {
		var err error
		workdir, err = createTemporaryWorkdir()
		if err != nil {
			return nil, err
		}
		logger.Info("Created temporary workdir", "path", workdir)
		defer deleteTemporaryWorkdir(workdir)
	} else {
		var err error
		workdir, err = filepath.Abs(workdir)
		if err != nil {
			return nil, err
		}
	}
	if err := installPulumiCLI(ctx, logger, workdir); err != nil {
		return nil, fmt.Errorf("error when installing Pulumi: %w", err)
	}
	os.Setenv("PATH", filepath.Join(workdir, "pulumi"))
	if err := installMigrateSnowflakeCLI(ctx, logger, workdir); err != nil {
		return nil, fmt.Errorf("error when installing Migrate Snowflake: %w", err)
	}

	warehouseName := m.warehouseName
	if !destroy {
		logger.Info("Copying database migrations to disk")
		if err := writeMigrationsToDisk(database.Migrations, database.MigrationsPath, filepath.Join(workdir, migrationsDir)); err != nil {
			return nil, err
		}
		logger.Info("Copied database migrations to disk")

		if warehouseName == "" {
			dsn, _, err := sf.GetDSN()
			if err != nil {
				return nil, fmt.Errorf("failed to create DSN: %w", err)
			}

			db, err := sql.Open("snowflake", dsn)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to Snowflake: %w", err)
			}
			defer db.Close()
			temporaryWarehouse := newTemporaryWarehouse(sf.NewClient(db, logger), logger)
			warehouseName = temporaryWarehouse.Name()
			if err := temporaryWarehouse.Create(ctx); err != nil {
				return nil, err
			}
			defer func() {
				if err := temporaryWarehouse.Delete(ctx); err != nil {
					logger.Error(err, "Failed to delete temporary warehouse, please do it manually", "name", warehouseName)
				}
			}()
		}
	}

	plugins := []pulumiPlugin{
		{name: "aws", version: pulumiAWSPluginVersion},
		{name: "snowflake", version: pulumiSnowflakePluginVersion},
		{name: "random", version: pulumiRandomPluginVersion},
		{name: "command", version: pulumiCommandPluginVersion},
	}

	s, err := m.setup(ctx, m.stackName, workdir, plugins, declareStack(warehouseName))
	if err != nil {
		return nil, err
	}

	destroyFunc := func() error {
		logger.Info("Destroying stack")
		var progressStream io.Writer
		if m.verbose {
			// wire up our destroy to stream progress to stdout
			progressStream = os.Stdout
		} else {
			progressStream = io.Discard
		}
		if _, err := s.Destroy(ctx, optdestroy.ProgressStreams(progressStream)); err != nil {
			return err
		}
		logger.Info("Destroyed stack")
		logger.Info("Removing stack")
		if err := s.Workspace().RemoveStack(ctx, s.Name()); err != nil {
			return err
		}
		logger.Info("Removed stack")
		return nil
	}

	if destroy {
		if err := destroyFunc(); err != nil {
			return nil, err
		}
		// return early
		return &Result{}, nil
	}

	updateFunc := func() (auto.UpResult, error) {
		logger.Info("Updating stack")
		var progressStream io.Writer
		if m.verbose {
			// wire up our update to stream progress to stdout
			progressStream = os.Stdout
		} else {
			progressStream = io.Discard
		}
		res, err := s.Up(ctx, optup.ProgressStreams(progressStream))
		if err != nil {
			return res, err
		}
		logger.Info("Updated stack")
		return res, nil
	}

	getStackOutputs := func(outs auto.OutputMap) (map[string]string, error) {
		result := make(map[string]string)
		names := []string{"bucketID", "databaseName", "storageIntegrationName", "notificationIntegrationName", "snsTopicARN", "sqsQueueARN"}
		for _, name := range names {
			v, ok := outs[name].Value.(string)
			if !ok {
				return nil, fmt.Errorf("failed to get '%s' from first stack outputs", name)
			}
			result[name] = v
		}
		return result, nil
	}

	upRes, err := updateFunc()
	if err != nil {
		return nil, err
	}
	outs, err := getStackOutputs(upRes.Outputs)
	if err != nil {
		return nil, err
	}

	return &Result{
		Region:            m.region,
		BucketName:        outs["bucketID"],
		BucketFlowsFolder: s3BucketFlowsFolder,
		DatabaseName:      outs["databaseName"],
		SchemaName:        schemaName,
		FlowsTableName:    flowsTableName,
		SNSTopicARN:       outs["snsTopicARN"],
		SQSQueueARN:       outs["sqsQueueARN"],
	}, nil
}

func (m *Manager) Onboard(ctx context.Context) (*Result, error) {
	return m.run(ctx, false)
}

func (m *Manager) Offboard(ctx context.Context) error {
	_, err := m.run(ctx, true)
	return err
}
