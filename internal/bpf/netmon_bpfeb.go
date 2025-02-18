// Code generated by bpf2go; DO NOT EDIT.
//go:build mips || mips64 || ppc64 || s390x

package bpf

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type netmonNetworkStats struct {
	BytesIn        uint64
	BytesOut       uint64
	PacketsIn      uint64
	PacketsOut     uint64
	TcpConnections uint32
	UdpConnections uint32
}

// loadNetmon returns the embedded CollectionSpec for netmon.
func loadNetmon() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_NetmonBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load netmon: %w", err)
	}

	return spec, err
}

// loadNetmonObjects loads netmon and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*netmonObjects
//	*netmonPrograms
//	*netmonMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadNetmonObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadNetmon()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// netmonSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type netmonSpecs struct {
	netmonProgramSpecs
	netmonMapSpecs
	netmonVariableSpecs
}

// netmonProgramSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type netmonProgramSpecs struct {
	TcEgress  *ebpf.ProgramSpec `ebpf:"tc_egress"`
	TcIngress *ebpf.ProgramSpec `ebpf:"tc_ingress"`
}

// netmonMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type netmonMapSpecs struct {
	InterfaceFilter *ebpf.MapSpec `ebpf:"interface_filter"`
	ProcessStats    *ebpf.MapSpec `ebpf:"process_stats"`
}

// netmonVariableSpecs contains global variables before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type netmonVariableSpecs struct {
}

// netmonObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadNetmonObjects or ebpf.CollectionSpec.LoadAndAssign.
type netmonObjects struct {
	netmonPrograms
	netmonMaps
	netmonVariables
}

func (o *netmonObjects) Close() error {
	return _NetmonClose(
		&o.netmonPrograms,
		&o.netmonMaps,
	)
}

// netmonMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadNetmonObjects or ebpf.CollectionSpec.LoadAndAssign.
type netmonMaps struct {
	InterfaceFilter *ebpf.Map `ebpf:"interface_filter"`
	ProcessStats    *ebpf.Map `ebpf:"process_stats"`
}

func (m *netmonMaps) Close() error {
	return _NetmonClose(
		m.InterfaceFilter,
		m.ProcessStats,
	)
}

// netmonVariables contains all global variables after they have been loaded into the kernel.
//
// It can be passed to loadNetmonObjects or ebpf.CollectionSpec.LoadAndAssign.
type netmonVariables struct {
}

// netmonPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadNetmonObjects or ebpf.CollectionSpec.LoadAndAssign.
type netmonPrograms struct {
	TcEgress  *ebpf.Program `ebpf:"tc_egress"`
	TcIngress *ebpf.Program `ebpf:"tc_ingress"`
}

func (p *netmonPrograms) Close() error {
	return _NetmonClose(
		p.TcEgress,
		p.TcIngress,
	)
}

func _NetmonClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed netmon_bpfeb.o
var _NetmonBytes []byte
