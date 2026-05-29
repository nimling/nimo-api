package test

import (
	"os"
	"testing"
	"github.com/nimling/nimo-api/internal"
)

func TestConvertCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "Convert OpenAPI spec to docs",
			args: []string{
				"../examples/spec.yml",
				"--docs", "../../tmp/test-docs",
				"--common-prefix", "test",
				"--write-introduction",
			},
			wantErr: false,
		},
		{
			name: "Convert OpenAPI spec to nginx",
			args: []string{
				"../examples/spec.yml",
				"-o", "../../tmp/test-nginx",
			},
			wantErr: false,
		},
		{
			name: "Convert with all options",
			args: []string{
				"../examples/spec.yml",
				"--docs", "../../tmp/test-all-docs",
				"-o", "../../tmp/test-all-nginx",
				"--common-prefix", "api",
				"--file-prefix", "example",
				"--write-introduction",
				"--merge-responses-inline",
			},
			wantErr: false,
		},
		{
			name: "Convert without input file",
			args: []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := internal.NewConvertCommand()
			cmd.SetArgs(tt.args)
			
			err := cmd.Execute()
			
			if (err != nil) != tt.wantErr {
				t.Errorf("convert command error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if !tt.wantErr && err == nil {
				for i, arg := range tt.args {
					if arg == "--docs" || arg == "-o" {
						if i+1 < len(tt.args) {
							dir := tt.args[i+1]
							if _, err := os.Stat(dir); os.IsNotExist(err) {
								t.Errorf("Expected output directory %s was not created", dir)
							}
						}
					}
				}
			}
		})
	}
	
	os.RemoveAll("../../tmp")
}

func TestRunConvertDirectly(t *testing.T) {
	args := []string{"../examples/spec.yml"}
	defer os.RemoveAll("../../tmp")

	err := internal.RunConvert(args, "", "../../tmp/test-direct", "", false, "", "", "", false, false, internal.InlineOptions{})
	if err != nil {
		t.Errorf("RunConvert failed: %v", err)
	}
}