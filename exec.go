package own

import (
	"fmt"
	"os/exec"
	"strings"

	ahoy_targets "gitlab.com/hidothealth/platform/ahoy/src/target"
)

type ExecScript struct {
	Pre     []string `mapstructure:"pre"`
	Command []string `mapstructure:"command"`
	Post    []string `mapstructure:"post"`
	Deps    []string `mapstructure:"deps"`
}

type ExecConfig struct {
	Outs                      []string              `mapstructure:"outs"`
	BuildCommand              []string              `mapstructure:"build"`
	Tools                     map[string]string     `mapstructure:"tools"`
	ScriptCommands            map[string]ExecScript `mapstructure:"scripts"`
	ExternalPath              *string               `mapstructure:"external_path"`
	ahoy_targets.BuildFields  `mapstructure:",squash"`
	ahoy_targets.DeployFields `mapstructure:",squash"`
}

func (ec ExecConfig) GetTargets(tcc *ahoy_targets.TargetConfigContext) ([]*ahoy_targets.Target, error) {
	opts := []ahoy_targets.TargetOption{
		ahoy_targets.WithSrcs(map[string][]string{"_srcs": ec.Srcs}),
		ahoy_targets.WithOuts(ec.Outs),
		ahoy_targets.WithVisibility(ec.Visibility),
		ahoy_targets.WithEnvVars(ec.Env),
		ahoy_targets.WithTools(ec.Tools),
		ahoy_targets.WithPassEnv(ec.PassEnv),
		ahoy_targets.WithEnvironments(ec.Environments),
		ahoy_targets.WithTargetScript("build", &ahoy_targets.TargetScript{
			Deps: ec.Deps,
			Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
				cmds := []string{}
				for _, cmd := range ec.BuildCommand {
					interpolatedCmd, err := target.Interpolate(cmd)
					if err != nil {
						return fmt.Errorf("interpolating cmd: %w", err)
					}
					cmds = append(cmds, interpolatedCmd)
				}

				return getCmd(cmds)(target, runCtx)
			},
		}),
	}

	if ec.ExternalPath != nil {
		interpolatedPath, err := tcc.Interpolate(*ec.ExternalPath)
		if err != nil {
			return nil, fmt.Errorf("interpolating external path (%s): %w", *ec.ExternalPath, err)
		}
		opts = append(opts, ahoy_targets.WithExternalPath(interpolatedPath))
	}

	for script, execCmd := range ec.ScriptCommands {
		if len(execCmd.Command) == 0 {
			return nil, fmt.Errorf("no commands provided for %s", script)
		}

		ts := &ahoy_targets.TargetScript{
			Deps: execCmd.Deps,
			Run:  getCmd(execCmd.Command),
		}
		if execCmd.Pre != nil {
			ts.Pre = getCmd(execCmd.Pre)
		}

		if execCmd.Post != nil {
			ts.Pre = getCmd(execCmd.Post)
		}

		opts = append(opts, ahoy_targets.WithTargetScript(script, ts))
	}

	return []*ahoy_targets.Target{
		ahoy_targets.NewTarget(
			ec.Name,
			opts...,
		),
	}, nil
}

func getCmd(scriptCmds []string) func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
	return func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
		target.SetStatus("Executing %s", target.Qn())
		args := []string{
			"-c",
		}

		for _, cmd := range scriptCmds {
			interpolatedCmd, err := target.Interpolate(cmd)
			if err != nil {
				return fmt.Errorf("interpolating cmd: %w", err)
			}
			args = append(args, interpolatedCmd)
		}

		target.Traceln("sh %s", strings.Join(args, " "))
		cmd := exec.Command("sh", args...)
		cmd.Dir = target.Cwd
		cmd.Env = target.GetEnvironmentVariablesList()
		cmd.Stdout = target
		cmd.Stderr = target
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("executing: %w", err)
		}
		return nil
	}
}
