// Copyright (2012) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.
//
//go:generate ../../bin/vmconfiger -type BaseConfig,KVMConfig,ContainerConfig

package main

import (
	"bridge"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// VMConfig contains all the configs possible for a VM. When a VM of a
// particular kind is launched, only the pertinent configuration is copied so
// fields from other configs will have the zero value for the field type.
type VMConfig struct {
	BaseConfig
	KVMConfig
	ContainerConfig
}

type ConfigWriter interface {
	WriteConfig(io.Writer) error
}

// BaseConfig contains all fields common to all VM types.
type BaseConfig struct {
	// Configures the UUID for a virtual machine. If not set, the VM will be
	// given a random one when it is launched.
	UUID string

	// Configures the number of virtual CPUs to allocate for a VM.
	//
	// Default: 1
	VCPUs uint64

	// Configures the amount of physical memory to allocate (in megabytes).
	//
	// Default: 2048
	Memory uint64

	// Enable or disable snapshot mode for disk images and container
	// filesystems. When enabled, disks/filesystems will be loaded in memory
	// when run and changes will not be saved. This allows a single
	// disk/filesystem to be used for many VMs.
	//
	// Default: true
	Snapshot bool

	// Set a host where the VM should be scheduled.
	//
	// Note: Cannot specify Schedule and Colocate in the same config.
	Schedule string `validate:"validSchedule"`

	// Colocate this VM with another VM that has already been launched or is
	// queued for launching.
	//
	// Note: Cannot specify Colocate and Schedule in the same
	Colocate string `validate:"validColocate"`

	// Set a limit on the number of VMs that should be scheduled on the same
	// host as the VM. A limit of zero means that the VM should be scheduled by
	// itself. A limit of -1 means that there is no limit. This is only used
	// when launching VMs in a namespace.
	//
	// Default: -1
	Coschedule int64

	// Enable/disable serial command and control layer for this VM.
	//
	// Default: true
	Backchannel bool

	// Networks for the VM, handler is not generated by vmconfiger.
	Networks NetConfigs

	// Set tags in the same manner as "vm tag". These tags will apply to all
	// newly launched VMs.
	Tags map[string]string
}

func (old VMConfig) Copy() VMConfig {
	return VMConfig{
		BaseConfig:      old.BaseConfig.Copy(),
		KVMConfig:       old.KVMConfig.Copy(),
		ContainerConfig: old.ContainerConfig.Copy(),
	}
}

func (vm VMConfig) String(namespace string) string {
	return vm.BaseConfig.String(namespace) +
		vm.KVMConfig.String() +
		vm.ContainerConfig.String()
}

func (vm *VMConfig) Clear(mask string) {
	vm.BaseConfig.Clear(mask)
	vm.KVMConfig.Clear(mask)
	vm.ContainerConfig.Clear(mask)
}

func (vm *VMConfig) WriteConfig(w io.Writer) error {
	funcs := []func(io.Writer) error{
		vm.BaseConfig.WriteConfig,
		vm.KVMConfig.WriteConfig,
		vm.ContainerConfig.WriteConfig,
	}

	for _, fn := range funcs {
		if err := fn(w); err != nil {
			return err
		}
	}

	return nil
}

func (old BaseConfig) Copy() BaseConfig {
	// Copy all fields
	res := old

	// Make deep copy of slices
	res.Networks = make([]NetConfig, len(old.Networks))
	copy(res.Networks, old.Networks)

	// Make deep copy of tags
	res.Tags = map[string]string{}
	for k, v := range old.Tags {
		res.Tags[k] = v
	}

	return res
}

func (vm *BaseConfig) String(namespace string) string {
	// create output
	var o bytes.Buffer
	fmt.Fprintln(&o, "VM configuration:")
	w := new(tabwriter.Writer)
	w.Init(&o, 5, 0, 1, ' ', 0)
	fmt.Fprintf(w, "Memory:\t%v\n", vm.Memory)
	fmt.Fprintf(w, "VCPUs:\t%v\n", vm.VCPUs)
	fmt.Fprintf(w, "Networks:\t%v\n", vm.NetworkString(namespace))
	fmt.Fprintf(w, "Snapshot:\t%v\n", vm.Snapshot)
	fmt.Fprintf(w, "UUID:\t%v\n", vm.UUID)
	fmt.Fprintf(w, "Schedule host:\t%v\n", vm.Schedule)
	fmt.Fprintf(w, "Coschedule limit:\t%v\n", vm.Coschedule)
	fmt.Fprintf(w, "Colocate:\t%v\n", vm.Colocate)
	fmt.Fprintf(w, "Backchannel:\t%v\n", vm.Backchannel)
	if vm.Tags != nil {
		fmt.Fprintf(w, "Tags:\t%v\n", marshal(vm.Tags))
	} else {
		fmt.Fprint(w, "Tags:\t{}\n")
	}
	w.Flush()
	fmt.Fprintln(&o)
	return o.String()
}

func (vm *BaseConfig) NetworkString(namespace string) string {
	return fmt.Sprintf("[%s]", vm.Networks.String())
}

func (vm *BaseConfig) QosString(b, t, i string) string {
	var val string
	br, err := getBridge(b)
	if err != nil {
		return val
	}

	ops := br.GetQos(t)
	if ops == nil {
		return ""
	}

	val += fmt.Sprintf("%s: ", i)
	for _, op := range ops {
		if op.Type == bridge.Delay {
			val += fmt.Sprintf("delay %s ", op.Value)
		}
		if op.Type == bridge.Loss {
			val += fmt.Sprintf("loss %s ", op.Value)
		}
		if op.Type == bridge.Rate {
			val += fmt.Sprintf("rate %s ", op.Value)
		}
	}
	return strings.Trim(val, " ")
}

func validSchedule(vmConfig VMConfig, s string) error {
	if vmConfig.Colocate != "" && s != "" {
		return errors.New("cannot specify schedule and colocate in the same config")
	}

	// check if s is in the namespace
	ns := GetNamespace()

	if !ns.Hosts[s] {
		return fmt.Errorf("host is not in namespace: %v", s)
	}

	return nil
}

func validColocate(vmConfig VMConfig, s string) error {
	if vmConfig.Schedule != "" && s != "" {
		return errors.New("cannot specify colocate and schedule in the same config")
	}

	// TODO: could check if s is a known VM
	return nil
}
