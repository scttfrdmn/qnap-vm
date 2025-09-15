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