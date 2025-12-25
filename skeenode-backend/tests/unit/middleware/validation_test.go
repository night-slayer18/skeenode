package middleware_test

import (
	"testing"

	. "skeenode/pkg/api/middleware"
)

func TestValidator_ValidateCommand_AcceptsNormalCommands(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	
	tests := []string{
		"echo hello",
		"ls -la",
		"python script.py --arg=value",
		"curl https://api.example.com",
	}
	
	for _, cmd := range tests {
		if err := v.ValidateCommand(cmd); err != nil {
			t.Errorf("expected command '%s' to be valid, got error: %v", cmd, err)
		}
	}
}

func TestValidator_ValidateCommand_RejectsDangerousCommands(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	
	tests := []string{
		"rm -rf /",
		"sudo rm -rf /",
		":(){ :|:& };:",  // Fork bomb
		"mkfs.ext4 /dev/sda",
	}
	
	for _, cmd := range tests {
		if err := v.ValidateCommand(cmd); err == nil {
			t.Errorf("expected command '%s' to be rejected", cmd)
		}
	}
}

func TestValidator_ValidateCommand_RejectsTooLong(t *testing.T) {
	config := DefaultValidatorConfig()
	config.MaxCommandLength = 10
	v := NewValidator(config)
	
	err := v.ValidateCommand("this is a very long command")
	if err == nil {
		t.Error("expected error for too long command")
	}
}

func TestValidator_ValidateJobType_AcceptsAllowed(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	
	for _, jobType := range []string{"SHELL", "DOCKER", "HTTP"} {
		if err := v.ValidateJobType(jobType); err != nil {
			t.Errorf("expected job type '%s' to be valid", jobType)
		}
	}
}

func TestValidator_ValidateJobType_RejectsUnknown(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	
	if err := v.ValidateJobType("UNKNOWN"); err == nil {
		t.Error("expected UNKNOWN job type to be rejected")
	}
}

func TestValidator_ValidateName_RejectsEmpty(t *testing.T) {
	v := NewValidator(DefaultValidatorConfig())
	
	if err := v.ValidateName(""); err == nil {
		t.Error("expected empty name to be rejected")
	}
}

func TestValidator_ValidateName_RejectsTooLong(t *testing.T) {
	config := DefaultValidatorConfig()
	config.MaxNameLength = 5
	v := NewValidator(config)
	
	if err := v.ValidateName("toolongname"); err == nil {
		t.Error("expected too long name to be rejected")
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "command",
		Message: "is required",
	}
	
	expected := "command: is required"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}
