package main

import (
	"net"
	"os"
	"path/filepath"

	"github.com/concourse/atc/atccmd"
	"github.com/concourse/tsa/tsacmd"
	"github.com/concourse/tsa/tsaflags"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"github.com/concourse/bin/bindata"
)

type WebCommand struct {
	atccmd.ATCCommand

	tsacmd.TSACommand `group:"TSA Configuration" namespace:"tsa"`
}

const cliArtifactsBindata = "cli-artifacts"

func (cmd WebCommand) lessenRequirements(command *flags.Command) {
	// defaults to address from external URL
	command.FindOptionByLongName("tsa-peer-ip").Required = false

	// defaults to atc external URL
	command.FindOptionByLongName("tsa-atc-url").Required = false

	// defaults to atc session signing key
	command.FindOptionByLongName("tsa-session-signing-key").Required = false
}

func (cmd *WebCommand) Execute(args []string) error {
	err := bindata.RestoreAssets(os.TempDir(), cliArtifactsBindata)
	if err != nil {
		return err
	}

	cmd.ATCCommand.CLIArtifactsDir = atccmd.DirFlag(filepath.Join(os.TempDir(), cliArtifactsBindata))

	cmd.populateTSAFlagsFromATCFlags()

	atcRunner, err := cmd.ATCCommand.Runner(args)
	if err != nil {
		return err
	}

	tsaRunner, err := cmd.TSACommand.Runner(args)
	if err != nil {
		return err
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, grouper.Members{
		{"atc", atcRunner},
		{"tsa", tsaRunner},
	}))

	return <-ifrit.Invoke(runner).Wait()
}

func (cmd *WebCommand) populateTSAFlagsFromATCFlags() error {
	var f tsaflags.URLFlag
	err := f.UnmarshalFlag(cmd.ATCCommand.PeerURL.String())
	if err != nil {
		return err
	}

	if len(cmd.TSACommand.ATCURLs) == 0 {
		cmd.TSACommand.ATCURLs = append(cmd.TSACommand.ATCURLs, f)
	}

	cmd.TSACommand.SessionSigningKeyPath = tsaflags.FileFlag(cmd.ATCCommand.SessionSigningKey)

	host, _, err := net.SplitHostPort(cmd.ATCCommand.PeerURL.URL().Host)
	if err != nil {
		return err
	}

	cmd.TSACommand.PeerIP = host

	cmd.TSACommand.Metrics.YellerAPIKey = cmd.ATCCommand.Metrics.YellerAPIKey
	cmd.TSACommand.Metrics.YellerEnvironment = cmd.ATCCommand.Metrics.YellerEnvironment

	return nil
}
