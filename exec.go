package own

import (
	"fmt"
	"os/exec"
	"strings"

	environs "github.com/zen-io/zen-core/environments"
	zen_targets "github.com/zen-io/zen-core/target"
)

type ExecScript struct {
	Pre     []string `mapstructure:"pre"`
	Command []string `mapstructure:"command"`
	Post    []string `mapstructure:"post"`
	Deps    []string `mapstructure:"deps"`
}

type ExecConfig struct {
	BuildCommand   []string                         `mapstructure:"build"`
	Srcs           []string                         `mapstructure:"srcs" desc:"Sources for the build"`
	MappedSrcs     map[string][]string              `mapstructure:"mapped_srcs" desc:"a map of organised srcs"`
	Outs           []string                         `mapstructure:"outs" desc:"Outs for the build"`
	ScriptCommands map[string]ExecScript            `mapstructure:"scripts"`
	ExternalPath   *string                          `mapstructure:"external_path"`
	Name           string                           `mapstructure:"name" desc:"Name for the target"`
	Description    string                           `mapstructure:"desc" desc:"Target description"`
	Labels         []string                         `mapstructure:"labels" desc:"Labels to apply to the targets"` //
	Deps           []string                         `mapstructure:"deps" desc:"Build dependencies"`
	PassEnv        []string                         `mapstructure:"pass_env" desc:"List of environment variable names that will be passed from the OS environment, they are part of the target hash"`
	SecretEnv      []string                         `mapstructure:"secret_env" desc:"List of environment variable names that will be passed from the OS environment, they are not used to calculate the target hash"`
	Env            map[string]string                `mapstructure:"env" desc:"Key-Value map of static environment variables to be used"`
	Tools          map[string]string                `mapstructure:"tools" desc:"Key-Value map of tools to include when executing this target. Values can be references"`
	Visibility     []string                         `mapstructure:"visibility" desc:"List of visibility for this target"`
	Environments   map[string]*environs.Environment `mapstructure:"environments" desc:"Deployment Environments"`
}

func (ec ExecConfig) GetTargets(tcc *zen_targets.TargetConfigContext) ([]*zen_targets.Target, error) {
	opts := []zen_targets.TargetOption{
		zen_targets.WithSrcs(map[string][]string{"_srcs": ec.Srcs}),
		zen_targets.WithOuts(ec.Outs),
		zen_targets.WithVisibility(ec.Visibility),
		zen_targets.WithEnvVars(ec.Env),
		zen_targets.WithTools(ec.Tools),
		zen_targets.WithPassEnv(ec.PassEnv),
		zen_targets.WithEnvironments(ec.Environments),
		zen_targets.WithTargetScript("build", &zen_targets.TargetScript{
			Deps: ec.Deps,
			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
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
		opts = append(opts, zen_targets.WithExternalPath(interpolatedPath))
	}

	for script, execCmd := range ec.ScriptCommands {
		if len(execCmd.Command) == 0 {
			return nil, fmt.Errorf("no commands provided for %s", script)
		}

		ts := &zen_targets.TargetScript{
			Deps: execCmd.Deps,
			Run:  getCmd(execCmd.Command),
		}
		if execCmd.Pre != nil {
			ts.Pre = getCmd(execCmd.Pre)
		}

		if execCmd.Post != nil {
			ts.Pre = getCmd(execCmd.Post)
		}

		opts = append(opts, zen_targets.WithTargetScript(script, ts))
	}

	return []*zen_targets.Target{
		zen_targets.NewTarget(
			ec.Name,
			opts...,
		),
	}, nil
}

func getCmd(scriptCmds []string) func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
	return func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
		target.SetStatus("Executing %s", target.Qn())

		args := []string{}
		for _, cmd := range scriptCmds {
			interpolatedCmd, err := target.Interpolate(cmd)
			if err != nil {
				return fmt.Errorf("interpolating cmd: %w", err)
			}
			args = append(args, interpolatedCmd)
		}
		cmdArg := []string{"-c", strings.Join(args, " && ")}

		target.Traceln("/bin/sh '%s'", strings.Join(cmdArg, " "))
		cmd := exec.Command("/bin/sh", cmdArg...)
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
