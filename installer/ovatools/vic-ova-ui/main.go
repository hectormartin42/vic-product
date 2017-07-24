// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// +build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	ui "github.com/gizak/termui"

	"github.com/dustin/go-humanize"

	"github.com/vmware/vic-product/installer/lib"
	"github.com/vmware/vic-product/installer/pkg/ip"
	"github.com/vmware/vic-product/installer/pkg/version"
	"github.com/vmware/vmw-guestinfo/vmcheck"
)

const (
	VT_ACTIVATE   = 0x5606
	VT_WAITACTIVE = 0x5607
)

func main() {

	// If we're running under linux, switch to virtual terminal 2 on startup
	ioctl(uintptr(os.Stdout.Fd()), VT_ACTIVATE, 2)
	ioctl(uintptr(os.Stdout.Fd()), VT_WAITACTIVE, 2)

	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

	gray := ui.ColorRGB(1, 1, 1)
	blue := ui.ColorCyan

	// Check if we're running inside a VM
	if isVM, err := vmcheck.IsVirtualWorld(); err != nil || !isVM {
		fmt.Fprintln(os.Stderr, "not living in a virtual world... :(")
		os.Exit(-1)
	}

	var info string

	ovf, err := lib.UnmarshaledOvfEnv()
	if err != nil {
		switch err.(type) {
		case lib.EnvFetchError:
			fmt.Println("impossible to fetch ovf environment, exiting")
			os.Exit(1)
		case lib.UnmarshalError:
			info = fmt.Sprintf("error: %s\n", err.Error())
		}
	}

	if ip, err := ip.FirstIPv4(ip.Eth0Interface); err == nil {
		if strings.ToLower(ovf.Properties["registry.deploy"]) == "true" {
			info = fmt.Sprintf("%sAccess the Container Registry at:\nhttps://%s:%s\n", info, ip.String(), ovf.Properties["registry.port"])
		}
		if strings.ToLower(ovf.Properties["management_portal.deploy"]) == "true" {
			info = fmt.Sprintf("%sAccess the Container Management Portal at:\nhttps://%s:%s\n", info, ip.String(), ovf.Properties["management_portal.port"])
		}
		if port, ok := ovf.Properties["fileserver.port"]; ok {
			info = fmt.Sprintf("%sAccess the fileserver at:\nhttps://%s:%s\n", info, ip.String(), port)
		}
		if port, ok := ovf.Properties["engine_installer.port"]; ok {
			info = fmt.Sprintf("%sAccess the Demo VCH Installer at:\nhttps://%s:%s\n", info, ip.String(), port)
		}
		if port, ok := ovf.Properties["cluster_manager.port"]; ok {
			info = fmt.Sprintf("%sAccess the Kubernetes on Vsphere daemon at:\nhttps://%s:%s\n", info, ip.String(), port)
		}
	}
	info = fmt.Sprintf("%s\nAccess the VIC Product Documentation at:\nhttps://vmware.github.io/vic-product/#documentation\n", info)
	info = fmt.Sprintf("%s\n\nPress the right arrow key to view network status...", info)

	toppanel := ui.NewPar(fmt.Sprintf("VMware vSphere Integrated Containers %s\n\n%s\n%s", version.GetBuild().ShortVersion(), getCPUs(), getMemory()))
	toppanel.Height = ui.TermHeight()/2 + 1
	toppanel.Width = ui.TermWidth()
	toppanel.TextFgColor = ui.ColorWhite
	toppanel.Y = 0
	toppanel.X = 0
	toppanel.TextBgColor = gray
	toppanel.Bg = gray
	toppanel.BorderBg = gray
	toppanel.BorderFg = ui.ColorWhite
	toppanel.BorderBottom = false
	toppanel.PaddingTop = 4
	toppanel.PaddingLeft = 4

	bottompanel := ui.NewPar(info)
	bottompanel.Height = ui.TermHeight()/2 + 1
	bottompanel.Width = ui.TermWidth()
	bottompanel.TextFgColor = ui.ColorBlack
	bottompanel.TextBgColor = blue
	bottompanel.Y = ui.TermHeight() / 2
	bottompanel.X = 0
	bottompanel.Bg = blue
	bottompanel.BorderFg = ui.ColorWhite
	bottompanel.BorderBg = blue
	bottompanel.BorderTop = false
	bottompanel.PaddingTop = 1
	bottompanel.PaddingLeft = 4

	netstat := &NetworkStatus{
		down:     "[DOWN](bg-red)",
		up:       "[UP](bg-green)",
		ovfProps: ovf.Properties,
	}

	netInto := fmt.Sprintf("Network Status:\n\nDNS: %s\n\nIP: %s\n\nGateway: %s\n", netstat.GetDNSStatus(), netstat.GetIPStatus(), netstat.GetGatewayStatus())
	netInto = fmt.Sprintf("%s\n\n\n\n\n\n\n\nPress the left arrow key to view service info...", netInto)

	// yellow := ui.ColorRGB(4, 4, 1)
	networkPanel := ui.NewPar(netInto)
	networkPanel.Height = ui.TermHeight()/2 + 1
	networkPanel.Width = ui.TermWidth()
	networkPanel.TextFgColor = ui.ColorBlack
	networkPanel.TextBgColor = ui.ColorWhite
	networkPanel.X = 0
	networkPanel.Y = ui.TermHeight() / 2
	networkPanel.Bg = ui.ColorWhite
	networkPanel.BorderBg = ui.ColorWhite
	networkPanel.BorderFg = ui.ColorBlack
	networkPanel.BorderTop = false
	networkPanel.PaddingTop = 1
	networkPanel.PaddingLeft = 4

	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})

	ui.Handle("/sys/kbd/<left>", func(ui.Event) {
		ui.Render(toppanel, bottompanel)
	})

	ui.Handle("/sys/kbd/<right>", func(ui.Event) {
		ui.Render(toppanel, networkPanel)
	})

	ui.Render(toppanel, bottompanel)

	ui.Loop()
}

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if e != 0 {
		return e
	}
	return nil
}

func getCPUs() string {
	out, _ := exec.Command("lscpu").Output()
	outstring := strings.TrimSpace(string(out))
	lines := strings.Split(outstring, "\n")
	var cpus string
	var model string
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "CPU(s)":
			cpus = value
		case "Model name":
			model = value
		}
	}

	return fmt.Sprintf("%sx %s", cpus, model)
}

func getMemory() string {
	si := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(si)
	if err != nil {
		panic("Austin, we have a problem... syscall.Sysinfo:" + err.Error())
	}
	return fmt.Sprintf("%s Memory", humanize.IBytes(uint64(si.Totalram)))
}