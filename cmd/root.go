// Package cmd provides the command-line interface for qnap-vm.
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scttfrdmn/qnap-vm/pkg/config"
	"github.com/scttfrdmn/qnap-vm/pkg/ssh"
	"github.com/scttfrdmn/qnap-vm/pkg/storage"
	"github.com/scttfrdmn/qnap-vm/pkg/virsh"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "qnap-vm",
	Short: "A CLI tool for managing virtual machines on QNAP devices",
	Long: `qnap-vm is a command-line tool for managing virtual machines on QNAP devices
with Virtualization Station. It provides easy-to-use commands for VM lifecycle
management, configuration, and monitoring.`,
	Version: version,
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringP("host", "H", "", "QNAP host address")
	rootCmd.PersistentFlags().StringP("username", "u", "", "SSH username")
	rootCmd.PersistentFlags().IntP("port", "p", 22, "SSH port")
	rootCmd.PersistentFlags().StringP("keyfile", "k", "", "SSH private key file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(
		listCmd(),
		createCmd(),
		startCmd(),
		stopCmd(),
		deleteCmd(),
		statusCmd(),
		snapshotCmd(),
		statsCmd(),
		cloneCmd(),
		consoleCmd(),
		configCmd(),
		versionCmd(),
	)
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets version information for the application
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
	rootCmd.Version = v
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all virtual machines",
		Long:  "List all virtual machines on the QNAP device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// List VMs
			vms, err := virshClient.ListVMs()
			if err != nil {
				return fmt.Errorf("failed to list VMs: %w", err)
			}

			if len(vms) == 0 {
				fmt.Println("No virtual machines found.")
				return nil
			}

			// Display VMs in a table format
			fmt.Printf("%-5s %-20s %-12s %-8s %-8s\n", "ID", "NAME", "STATE", "MEMORY", "CPUS")
			fmt.Printf("%-5s %-20s %-12s %-8s %-8s\n", "-----", "--------------------", "------------", "--------", "--------")

			for _, vm := range vms {
				// Get detailed info for each VM
				if detailed, err := virshClient.GetVMDetails(vm.Name); err == nil {
					vm = *detailed
				}

				idStr := "-"
				if vm.ID > 0 {
					idStr = fmt.Sprintf("%d", vm.ID)
				}

				memoryStr := "-"
				if vm.Memory > 0 {
					memoryStr = fmt.Sprintf("%dM", vm.Memory)
				}

				cpusStr := "-"
				if vm.CPUs > 0 {
					cpusStr = fmt.Sprintf("%d", vm.CPUs)
				}

				fmt.Printf("%-5s %-20s %-12s %-8s %-8s\n",
					idStr, vm.Name, vm.State, memoryStr, cpusStr)
			}

			return nil
		},
	}
}

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [VM_NAME]",
		Short: "Create a new virtual machine",
		Long:  "Create a new virtual machine with the specified configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]

			// Get command line arguments
			memoryStr, _ := cmd.Flags().GetString("memory")
			cpusStr, _ := cmd.Flags().GetString("cpus")
			diskSize, _ := cmd.Flags().GetString("disk")
			isoPath, _ := cmd.Flags().GetString("iso")

			// Parse memory and CPU values
			memory, err := strconv.Atoi(memoryStr)
			if err != nil {
				return fmt.Errorf("invalid memory value: %s", memoryStr)
			}

			cpus, err := strconv.Atoi(cpusStr)
			if err != nil {
				return fmt.Errorf("invalid CPU value: %s", cpusStr)
			}

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM already exists
			if _, err := virshClient.GetVM(vmName); err == nil {
				return fmt.Errorf("VM '%s' already exists", vmName)
			}

			// Detect storage and create disk
			storageManager := storage.NewManager(sshClient)
			pool, err := storageManager.GetBestPool()
			if err != nil {
				return fmt.Errorf("failed to find storage pool: %w", err)
			}

			fmt.Printf("Using storage pool: %s (%s)\n", pool.Name, pool.Path)

			// Create disk path and image
			diskPath := storageManager.CreateVMDiskPath(pool, vmName)
			fmt.Printf("Creating disk image: %s (%s)\n", diskPath, diskSize)

			if err := storageManager.CreateVMDisk(diskPath, diskSize); err != nil {
				return fmt.Errorf("failed to create disk: %w", err)
			}

			// Create VM configuration
			vmConfig := virsh.VMConfig{
				Memory:   memory,
				CPUs:     cpus,
				DiskSize: diskSize,
				DiskPath: diskPath,
				ISOPath:  isoPath,
			}

			fmt.Printf("Creating VM '%s' (Memory: %dMB, CPUs: %d)...\n", vmName, memory, cpus)

			// Create the VM
			if err := virshClient.CreateVM(vmName, vmConfig); err != nil {
				return fmt.Errorf("failed to create VM: %w", err)
			}

			fmt.Printf("VM '%s' created successfully!\n", vmName)
			fmt.Printf("Disk: %s\n", diskPath)
			if isoPath != "" {
				fmt.Printf("ISO: %s\n", isoPath)
			}

			return nil
		},
	}

	cmd.Flags().StringP("template", "t", "", "VM template to use")
	cmd.Flags().StringP("memory", "m", "2048", "Memory size in MB")
	cmd.Flags().StringP("cpus", "c", "2", "Number of CPU cores")
	cmd.Flags().StringP("disk", "d", "20G", "Disk size")
	cmd.Flags().StringP("iso", "i", "", "ISO file path for installation")

	return cmd
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [VM_NAME]",
		Short: "Start a virtual machine",
		Long:  "Start the specified virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists
			vm, err := virshClient.GetVM(vmName)
			if err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			if strings.Contains(vm.State, "running") {
				fmt.Printf("VM '%s' is already running\n", vmName)
				return nil
			}

			fmt.Printf("Starting VM '%s'...\n", vmName)
			if err := virshClient.StartVM(vmName); err != nil {
				return fmt.Errorf("failed to start VM: %w", err)
			}

			fmt.Printf("VM '%s' started successfully\n", vmName)
			return nil
		},
	}
}

func stopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [VM_NAME]",
		Short: "Stop a virtual machine",
		Long:  "Stop the specified virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists
			vm, err := virshClient.GetVM(vmName)
			if err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			if strings.Contains(vm.State, "shut off") {
				fmt.Printf("VM '%s' is already stopped\n", vmName)
				return nil
			}

			action := "Shutting down"
			if force {
				action = "Force stopping"
			}

			fmt.Printf("%s VM '%s'...\n", action, vmName)
			if err := virshClient.StopVM(vmName, force); err != nil {
				return fmt.Errorf("failed to stop VM: %w", err)
			}

			fmt.Printf("VM '%s' stopped successfully\n", vmName)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "Force stop the VM")

	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [VM_NAME]",
		Short: "Delete a virtual machine",
		Long:  "Delete the specified virtual machine and its associated resources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists
			_, err = virshClient.GetVM(vmName)
			if err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			// Confirmation unless force is used
			if !force {
				fmt.Printf("Are you sure you want to delete VM '%s'? This will permanently delete the VM and its disk. (y/N): ", vmName)
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to read input: %v\n", err)
				}
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			fmt.Printf("Deleting VM '%s'...\n", vmName)
			if err := virshClient.DeleteVM(vmName); err != nil {
				return fmt.Errorf("failed to delete VM: %w", err)
			}

			fmt.Printf("VM '%s' deleted successfully\n", vmName)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "Force delete without confirmation")

	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [VM_NAME]",
		Short: "Show VM status and resource usage",
		Long:  "Show detailed status and resource usage for the specified virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Get detailed VM information
			vm, err := virshClient.GetVMDetails(vmName)
			if err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			// Display VM status
			fmt.Printf("VM Status: %s\n", vmName)
			fmt.Printf("%-15s: %s\n", "State", vm.State)
			fmt.Printf("%-15s: %s\n", "UUID", vm.UUID)

			if vm.ID > 0 {
				fmt.Printf("%-15s: %d\n", "ID", vm.ID)
			}

			if vm.Memory > 0 {
				fmt.Printf("%-15s: %d MB\n", "Memory", vm.Memory)
			}

			if vm.CPUs > 0 {
				fmt.Printf("%-15s: %d\n", "CPUs", vm.CPUs)
			}

			return nil
		},
	}
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage connection configuration",
		Long:  "Manage QNAP device connection configuration",
	}

	// Config set command
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set configuration values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			host, _ := cmd.Flags().GetString("host")
			username, _ := cmd.Flags().GetString("username")
			port, _ := cmd.Flags().GetInt("port")
			keyfile, _ := cmd.Flags().GetString("keyfile")
			hostName, _ := cmd.Flags().GetString("name")

			if hostName == "" {
				hostName = "default"
			}

			// Load existing config
			configFile, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get existing host config or create new
			existingConfig, _ := configFile.GetHostConfig(hostName)

			// Update with provided values
			newConfig := existingConfig
			if host != "" {
				newConfig.Host = host
			}
			if username != "" {
				newConfig.Username = username
			}
			if port != 0 {
				newConfig.Port = port
			}
			if keyfile != "" {
				newConfig.KeyFile = keyfile
			}

			// Set defaults
			newConfig.SetDefaults()

			// Validate
			if err := newConfig.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			// Save configuration
			configFile.SetHostConfig(hostName, newConfig)
			if configFile.DefaultHost == "" {
				configFile.SetDefaultHost(hostName)
			}

			if err := config.SaveConfig(configFile); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Configuration saved for host '%s'\n", hostName)
			return nil
		},
	}

	setCmd.Flags().String("host", "", "QNAP host address")
	setCmd.Flags().String("username", "", "SSH username")
	setCmd.Flags().Int("port", 0, "SSH port")
	setCmd.Flags().String("keyfile", "", "SSH private key file")
	setCmd.Flags().String("name", "", "Configuration name (default: 'default')")

	// Config show command
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			hosts := configFile.ListHosts()
			if len(hosts) == 0 {
				fmt.Println("No configurations found. Use 'qnap-vm config set' to create one.")
				return nil
			}

			fmt.Printf("Default Host: %s\n\n", configFile.DefaultHost)
			fmt.Printf("%-15s %-25s %-15s %-6s %-30s\n", "NAME", "HOST", "USERNAME", "PORT", "KEYFILE")
			fmt.Printf("%-15s %-25s %-15s %-6s %-30s\n", "---------------", "-------------------------", "---------------", "------", "------------------------------")

			for _, hostName := range hosts {
				if hostConfig, exists := configFile.GetHostConfig(hostName); exists {
					keyFile := hostConfig.KeyFile
					if keyFile == "" {
						keyFile = "(default)"
					}

					fmt.Printf("%-15s %-25s %-15s %-6d %-30s\n",
						hostName, hostConfig.Host, hostConfig.Username, hostConfig.Port, keyFile)
				}
			}

			return nil
		},
	}

	cmd.AddCommand(setCmd, showCmd)
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("qnap-vm version %s\n", version)
			if commit != "none" {
				fmt.Printf("commit: %s\n", commit)
			}
			if date != "unknown" {
				fmt.Printf("built: %s\n", date)
			}
		},
	}
}

func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	// Load configuration file
	configFile, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Get host configuration (default or specified)
	var cfg config.Config
	if hostConfig, exists := configFile.GetHostConfig(""); exists {
		cfg = hostConfig
	}

	// Override with command line flags
	flagCfg := config.Config{}
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		flagCfg.Host = host
	}
	if username, _ := cmd.Flags().GetString("username"); username != "" {
		flagCfg.Username = username
	}
	if port, _ := cmd.Flags().GetInt("port"); port != 22 {
		flagCfg.Port = port
	}
	if keyfile, _ := cmd.Flags().GetString("keyfile"); keyfile != "" {
		flagCfg.KeyFile = keyfile
	}

	// Merge configurations (flags override config file)
	cfg = cfg.MergeWith(flagCfg)

	// Set defaults and validate
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// connectToQNAP establishes SSH connection and sets up virsh client
func connectToQNAP(cfg config.Config) (*ssh.Client, *virsh.Client, error) {
	// Create SSH client
	sshCfg := ssh.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		KeyFile:  cfg.KeyFile,
		Password: cfg.Password,
		Timeout:  30 * time.Second,
	}

	sshClient, err := ssh.NewClient(sshCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SSH client: %w", err)
	}

	// Connect to QNAP device
	if err := sshClient.Connect(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to QNAP device: %w", err)
	}

	// Test connection
	if err := sshClient.TestConnection(); err != nil {
		if closeErr := sshClient.Close(); closeErr != nil {
			return nil, nil, fmt.Errorf("SSH connection test failed: %w (close error: %v)", err, closeErr)
		}
		return nil, nil, fmt.Errorf("SSH connection test failed: %w", err)
	}

	// Create virsh client
	virshClient := virsh.NewClient(sshClient)

	// Initialize virsh environment
	if err := virshClient.Initialize(); err != nil {
		if closeErr := sshClient.Close(); closeErr != nil {
			return nil, nil, fmt.Errorf("failed to initialize virsh: %w (close error: %v)", err, closeErr)
		}
		return nil, nil, fmt.Errorf("failed to initialize virsh: %w", err)
	}

	return sshClient, virshClient, nil
}

func snapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage VM snapshots",
		Long:  "Create, list, restore, and delete virtual machine snapshots",
	}

	// Snapshot create command
	createSnapshotCmd := &cobra.Command{
		Use:   "create [VM_NAME] [SNAPSHOT_NAME]",
		Short: "Create a VM snapshot",
		Long:  "Create a snapshot of the specified virtual machine",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			snapshotName := args[1]
			description, _ := cmd.Flags().GetString("description")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists
			if _, err := virshClient.GetVM(vmName); err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			fmt.Printf("Creating snapshot '%s' for VM '%s'...\n", snapshotName, vmName)
			if err := virshClient.CreateSnapshot(vmName, snapshotName, description); err != nil {
				return fmt.Errorf("failed to create snapshot: %w", err)
			}

			fmt.Printf("Snapshot '%s' created successfully\n", snapshotName)
			if description != "" {
				fmt.Printf("Description: %s\n", description)
			}

			return nil
		},
	}

	createSnapshotCmd.Flags().StringP("description", "d", "", "Snapshot description")

	// Snapshot list command
	listSnapshotCmd := &cobra.Command{
		Use:   "list [VM_NAME]",
		Short: "List VM snapshots",
		Long:  "List all snapshots for the specified virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists
			if _, err := virshClient.GetVM(vmName); err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			// List snapshots
			snapshots, err := virshClient.ListSnapshots(vmName)
			if err != nil {
				return fmt.Errorf("failed to list snapshots: %w", err)
			}

			if len(snapshots) == 0 {
				fmt.Printf("No snapshots found for VM '%s'\n", vmName)
				return nil
			}

			// Get current snapshot
			currentSnapshot, _ := virshClient.GetCurrentSnapshot(vmName)

			// Display snapshots in table format
			fmt.Printf("Snapshots for VM '%s':\n\n", vmName)
			fmt.Printf("%-20s %-25s %-12s %-8s %-50s\n", "NAME", "CREATION TIME", "STATE", "CURRENT", "DESCRIPTION")
			fmt.Printf("%-20s %-25s %-12s %-8s %-50s\n", "--------------------", "-------------------------", "------------", "--------", "--------------------------------------------------")

			for _, snapshot := range snapshots {
				currentStr := ""
				if snapshot.Name == currentSnapshot {
					currentStr = "✓"
				}

				// Get detailed info for description
				if detailed, err := virshClient.GetSnapshotInfo(vmName, snapshot.Name); err == nil {
					snapshot = *detailed
				}

				description := snapshot.Description
				if len(description) > 50 {
					description = description[:47] + "..."
				}

				fmt.Printf("%-20s %-25s %-12s %-8s %-50s\n",
					snapshot.Name, snapshot.CreationTime, snapshot.State, currentStr, description)
			}

			return nil
		},
	}

	// Snapshot restore command
	restoreSnapshotCmd := &cobra.Command{
		Use:   "restore [VM_NAME] [SNAPSHOT_NAME]",
		Short: "Restore VM to snapshot",
		Long:  "Restore the specified virtual machine to a snapshot state",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			snapshotName := args[1]
			force, _ := cmd.Flags().GetBool("force")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM and snapshot exist
			if _, err := virshClient.GetVM(vmName); err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			if _, err := virshClient.GetSnapshotInfo(vmName, snapshotName); err != nil {
				return fmt.Errorf("snapshot '%s' not found for VM '%s'", snapshotName, vmName)
			}

			// Confirmation unless force is used
			if !force {
				fmt.Printf("⚠️  WARNING: Restoring VM '%s' to snapshot '%s' will lose all changes made after the snapshot.\n", vmName, snapshotName)
				fmt.Print("Are you sure you want to continue? (y/N): ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to read input: %v\n", err)
				}
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			fmt.Printf("Restoring VM '%s' to snapshot '%s'...\n", vmName, snapshotName)
			if err := virshClient.RestoreSnapshot(vmName, snapshotName); err != nil {
				return fmt.Errorf("failed to restore snapshot: %w", err)
			}

			fmt.Printf("VM '%s' restored to snapshot '%s' successfully\n", vmName, snapshotName)
			return nil
		},
	}

	restoreSnapshotCmd.Flags().BoolP("force", "f", false, "Force restore without confirmation")

	// Snapshot delete command
	deleteSnapshotCmd := &cobra.Command{
		Use:   "delete [VM_NAME] [SNAPSHOT_NAME]",
		Short: "Delete a VM snapshot",
		Long:  "Delete the specified snapshot from a virtual machine",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			snapshotName := args[1]
			force, _ := cmd.Flags().GetBool("force")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM and snapshot exist
			if _, err := virshClient.GetVM(vmName); err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			if _, err := virshClient.GetSnapshotInfo(vmName, snapshotName); err != nil {
				return fmt.Errorf("snapshot '%s' not found for VM '%s'", snapshotName, vmName)
			}

			// Confirmation unless force is used
			if !force {
				fmt.Printf("Are you sure you want to delete snapshot '%s' from VM '%s'? (y/N): ", snapshotName, vmName)
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to read input: %v\n", err)
				}
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			fmt.Printf("Deleting snapshot '%s' from VM '%s'...\n", snapshotName, vmName)
			if err := virshClient.DeleteSnapshot(vmName, snapshotName); err != nil {
				return fmt.Errorf("failed to delete snapshot: %w", err)
			}

			fmt.Printf("Snapshot '%s' deleted successfully\n", snapshotName)
			return nil
		},
	}

	deleteSnapshotCmd.Flags().BoolP("force", "f", false, "Force delete without confirmation")

	// Snapshot current command
	currentSnapshotCmd := &cobra.Command{
		Use:   "current [VM_NAME]",
		Short: "Show current snapshot",
		Long:  "Show the current snapshot for the specified virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists
			if _, err := virshClient.GetVM(vmName); err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			// Get current snapshot
			currentSnapshot, err := virshClient.GetCurrentSnapshot(vmName)
			if err != nil {
				return fmt.Errorf("failed to get current snapshot: %w", err)
			}

			if currentSnapshot == "" {
				fmt.Printf("VM '%s' has no current snapshot\n", vmName)
				return nil
			}

			// Get detailed snapshot info
			snapshotInfo, err := virshClient.GetSnapshotInfo(vmName, currentSnapshot)
			if err != nil {
				fmt.Printf("Current snapshot: %s\n", currentSnapshot)
				return nil
			}

			fmt.Printf("Current snapshot for VM '%s':\n", vmName)
			fmt.Printf("%-15s: %s\n", "Name", snapshotInfo.Name)
			fmt.Printf("%-15s: %s\n", "Creation Time", snapshotInfo.CreationTime)
			fmt.Printf("%-15s: %s\n", "State", snapshotInfo.State)
			if snapshotInfo.Description != "" {
				fmt.Printf("%-15s: %s\n", "Description", snapshotInfo.Description)
			}

			return nil
		},
	}

	cmd.AddCommand(createSnapshotCmd, listSnapshotCmd, restoreSnapshotCmd, deleteSnapshotCmd, currentSnapshotCmd)
	return cmd
}

func statsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats [VM_NAME]",
		Short: "Show VM resource statistics",
		Long:  "Show detailed resource usage statistics for the specified virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			watch, _ := cmd.Flags().GetBool("watch")
			interval, _ := cmd.Flags().GetInt("interval")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists and is running
			vm, err := virshClient.GetVM(vmName)
			if err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			if !strings.Contains(vm.State, "running") {
				return fmt.Errorf("VM '%s' is not running (state: %s)", vmName, vm.State)
			}

			// Display stats once or in watch mode
			if watch {
				fmt.Printf("Watching VM '%s' statistics (press Ctrl+C to exit)\n\n", vmName)
				for {
					if err := displayVMStats(virshClient, vmName); err != nil {
						return err
					}
					time.Sleep(time.Duration(interval) * time.Second)
					fmt.Print("\033[H\033[2J") // Clear screen
				}
			} else {
				return displayVMStats(virshClient, vmName)
			}
		},
	}

	cmd.Flags().BoolP("watch", "w", false, "Watch statistics in real-time")
	cmd.Flags().IntP("interval", "i", 5, "Update interval in seconds (for watch mode)")

	return cmd
}

func displayVMStats(virshClient *virsh.Client, vmName string) error {
	stats, err := virshClient.GetVMStats(vmName)
	if err != nil {
		return fmt.Errorf("failed to get VM statistics: %w", err)
	}

	fmt.Printf("VM Statistics: %s\n", vmName)
	fmt.Printf("%-20s: %s\n", "Timestamp", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	// CPU Statistics
	fmt.Printf("CPU:\n")
	fmt.Printf("  %-18s: %d ns\n", "CPU Time", stats.CPUTime)

	// Memory Statistics
	fmt.Printf("\nMemory:\n")
	if stats.Memory.Total > 0 {
		fmt.Printf("  %-18s: %s\n", "Total", formatBytes(stats.Memory.Total*1024))
		fmt.Printf("  %-18s: %s\n", "Used", formatBytes(stats.Memory.Used*1024))
		fmt.Printf("  %-18s: %s\n", "Available", formatBytes(stats.Memory.Available*1024))
		fmt.Printf("  %-18s: %.1f%%\n", "Usage", stats.Memory.Percent)
	} else {
		fmt.Printf("  %-18s: Not available\n", "Statistics")
	}

	// Block I/O Statistics
	fmt.Printf("\nDisk I/O:\n")
	fmt.Printf("  %-18s: %s\n", "Read", formatBytes(stats.BlockIO.ReadBytes))
	fmt.Printf("  %-18s: %s\n", "Written", formatBytes(stats.BlockIO.WriteBytes))
	fmt.Printf("  %-18s: %d\n", "Read Requests", stats.BlockIO.ReadReqs)
	fmt.Printf("  %-18s: %d\n", "Write Requests", stats.BlockIO.WriteReqs)

	// Network Statistics
	fmt.Printf("\nNetwork:\n")
	fmt.Printf("  %-18s: %s\n", "Received", formatBytes(stats.Network.RxBytes))
	fmt.Printf("  %-18s: %s\n", "Transmitted", formatBytes(stats.Network.TxBytes))
	fmt.Printf("  %-18s: %d\n", "RX Packets", stats.Network.RxPackets)
	fmt.Printf("  %-18s: %d\n", "TX Packets", stats.Network.TxPackets)

	return nil
}

// formatBytes formats byte values into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

func cloneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone [SOURCE_VM] [TARGET_VM]",
		Short: "Clone a virtual machine",
		Long:  "Clone an existing virtual machine to create a new VM with the same configuration",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			sourceVM := args[0]
			targetVM := args[1]
			linkedClone, _ := cmd.Flags().GetBool("linked")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if source VM exists
			sourceVMInfo, err := virshClient.GetVM(sourceVM)
			if err != nil {
				return fmt.Errorf("source VM '%s' not found", sourceVM)
			}

			// Check if target VM already exists
			if _, err := virshClient.GetVM(targetVM); err == nil {
				return fmt.Errorf("target VM '%s' already exists", targetVM)
			}

			cloneType := "full"
			if linkedClone {
				cloneType = "linked"
			}

			fmt.Printf("Cloning VM '%s' to '%s' (%s clone)...\n", sourceVM, targetVM, cloneType)
			fmt.Printf("Source VM state: %s\n", sourceVMInfo.State)

			if err := virshClient.CloneVM(sourceVM, targetVM, linkedClone); err != nil {
				return fmt.Errorf("failed to clone VM: %w", err)
			}

			fmt.Printf("VM '%s' cloned successfully to '%s'\n", sourceVM, targetVM)

			// Show the new VM info
			if newVM, err := virshClient.GetVMDetails(targetVM); err == nil {
				fmt.Printf("New VM details:\n")
				fmt.Printf("  Name: %s\n", newVM.Name)
				fmt.Printf("  State: %s\n", newVM.State)
				fmt.Printf("  Memory: %d MB\n", newVM.Memory)
				fmt.Printf("  CPUs: %d\n", newVM.CPUs)
				fmt.Printf("  UUID: %s\n", newVM.UUID)
			}

			return nil
		},
	}

	cmd.Flags().BoolP("linked", "l", false, "Create a linked clone (space-efficient)")

	return cmd
}

func consoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console [VM_NAME]",
		Short: "Access VM console",
		Long:  "Access virtual machine console via VNC or serial connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}

			vmName := args[0]
			vncOnly, _ := cmd.Flags().GetBool("vnc")
			serialOnly, _ := cmd.Flags().GetBool("serial")
			force, _ := cmd.Flags().GetBool("force")

			// Connect to QNAP device
			sshClient, virshClient, err := connectToQNAP(*cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := sshClient.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to close SSH connection: %v\n", err)
				}
			}()

			// Check if VM exists and is running
			vm, err := virshClient.GetVM(vmName)
			if err != nil {
				return fmt.Errorf("VM '%s' not found", vmName)
			}

			if !strings.Contains(vm.State, "running") {
				return fmt.Errorf("VM '%s' is not running (state: %s). Console access requires a running VM.", vmName, vm.State)
			}

			// Get console information
			consoleInfo, err := virshClient.GetConsoleInfo(vmName)
			if err != nil {
				return fmt.Errorf("failed to get console information: %w", err)
			}

			// Handle VNC access
			if vncOnly || (!serialOnly && consoleInfo.Protocol == "VNC") {
				vncConnection, err := virshClient.GetVNCConnectionString(vmName)
				if err != nil {
					return fmt.Errorf("failed to get VNC connection: %w", err)
				}

				fmt.Printf("VNC Console Access for VM '%s':\n\n", vmName)
				fmt.Printf("Connection Details:\n")
				fmt.Printf("  Protocol: %s\n", consoleInfo.Protocol)
				fmt.Printf("  Host: %s\n", consoleInfo.VNCHost)
				fmt.Printf("  Port: %d\n", consoleInfo.VNCPort)
				fmt.Printf("  Display: %s\n", consoleInfo.VNCDisplay)
				fmt.Printf("\nVNC Connection String: %s\n\n", vncConnection)

				fmt.Printf("To connect using a VNC client:\n")
				fmt.Printf("  vncviewer %s\n", vncConnection)
				fmt.Printf("  open vnc://%s  # macOS Screen Sharing\n", vncConnection)
				fmt.Printf("\nOr use SSH tunnel for secure access:\n")
				fmt.Printf("  ssh -L %d:localhost:%d %s@%s\n", consoleInfo.VNCPort, consoleInfo.VNCPort, cfg.Username, cfg.Host)
				fmt.Printf("  vncviewer localhost:%d\n", consoleInfo.VNCPort)

				return nil
			}

			// Handle serial console access
			if serialOnly || consoleInfo.SerialPort == "available" {
				fmt.Printf("Serial Console Access for VM '%s':\n\n", vmName)
				fmt.Printf("Note: Serial console requires proper guest OS configuration.\n")
				fmt.Printf("Guest OS must have:\n")
				fmt.Printf("  1. Serial console enabled in kernel parameters\n")
				fmt.Printf("  2. Getty service running on serial port\n")
				fmt.Printf("  3. Appropriate permissions configured\n\n")

				if !force {
					fmt.Print("Attempt to connect to serial console? This may require guest OS setup. (y/N): ")
					var response string
					if _, err := fmt.Scanln(&response); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to read input: %v\n", err)
					}
					if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
						fmt.Println("Console connection cancelled")
						return nil
					}
				}

				fmt.Printf("Connecting to serial console for VM '%s'...\n", vmName)
				fmt.Printf("Use 'Ctrl+]' to exit the console session.\n\n")

				// This would normally connect to interactive console
				// For CLI tool, we'll provide connection instructions instead
				fmt.Printf("To connect to serial console manually:\n")
				fmt.Printf("  ssh %s@%s\n", cfg.Username, cfg.Host)
				fmt.Printf("  virsh console %s\n", vmName)

				return nil
			}

			return fmt.Errorf("no console access available for VM '%s'", vmName)
		},
	}

	cmd.Flags().BoolP("vnc", "", false, "Show VNC console information only")
	cmd.Flags().BoolP("serial", "s", false, "Connect to serial console only")
	cmd.Flags().BoolP("force", "f", false, "Force console connection without confirmation")

	return cmd
}
