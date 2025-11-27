package test

import (
	"os"
	"testing"
	"github.com/nimling/nimo-api/internal"
)

func TestSyncCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		setup   func() error
		verify  func() error
		wantErr bool
	}{
		{
			name: "Sync with valid mapping",
			args: []string{
				"-s", "../examples/mapping.json",
			},
			setup: func() error {
				return os.Chdir("../examples")
			},
			verify: func() error {
				files := []string{
					"output/guides/myproject/index.md",
					"output/api/myproject/index.md",
					"output/tutorials/getting-started.md",
				}
				for _, file := range files {
					if _, err := os.Stat(file); os.IsNotExist(err) {
						return err
					}
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "Sync without mapping file",
			args: []string{},
			wantErr: true,
		},
		{
			name: "Sync with non-existent mapping file",
			args: []string{
				"-s", "non-existent.json",
			},
			wantErr: true,
		},
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Chdir(originalDir)
			
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}
			
			cmd := internal.NewSyncCommand()
			cmd.SetArgs(tt.args)
			
			err := cmd.Execute()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("sync command error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if tt.verify != nil && !tt.wantErr {
				if err := tt.verify(); err != nil {
					t.Errorf("Verification failed: %v", err)
				}
			}
			
			os.RemoveAll("output")
		})
	}
}

func TestRunSyncDirectly(t *testing.T) {
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	
	os.Chdir("../examples")
	defer os.RemoveAll("output")
	
	err := internal.RunSync("mapping.json")
	if err != nil {
		t.Errorf("RunSync failed: %v", err)
	}
}