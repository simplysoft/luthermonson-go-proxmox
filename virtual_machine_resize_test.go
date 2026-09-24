package proxmox

import (
	"context"
	"net/http"
	"testing"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const resizeTestUPID = "UPID:pve:00001234:0000ABCD:68D3A1B2:resize:scsi0:root@pam:"

func TestVirtualMachine_ResizeDiskReturnsStartedTask(t *testing.T) {
	defer gock.Off()

	gock.New(TestURI).
		Put("/nodes/pve/qemu/101/resize").
		MatchType("json").
		JSON(map[string]string{"disk": "scsi0", "size": "40G"}).
		Reply(http.StatusOK).
		JSON(map[string]string{"data": resizeTestUPID})

	client := mockClient()
	vm := VirtualMachine{client: client, VMID: 101, Node: "pve"}

	task, err := vm.ResizeDisk(context.Background(), "scsi0", "40G")

	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Same(t, client, task.client)
	assert.Equal(t, UPID(resizeTestUPID), task.UPID)
	assert.Equal(t, "pve", task.Node)
	assert.Equal(t, "resize", task.Type)
	assert.True(t, gock.IsDone())
}

func TestVirtualMachine_ResizeDiskErrorPaths(t *testing.T) {
	defer gock.Off()

	gock.New(TestURI).
		Put("/nodes/pve/qemu/102/resize").
		Reply(http.StatusBadRequest).
		JSON(map[string]string{"data": "no resize for you"})

	client := mockClient()
	vm := VirtualMachine{client: client, VMID: 102, Node: "pve"}
	task, err := vm.ResizeDisk(context.Background(), "scsi0", "40G")
	assert.Error(t, err)
	assert.Nil(t, task)

	gock.New(TestURI).
		Put("/nodes/pve/qemu/102/resize").
		Reply(http.StatusOK).
		JSON(map[string]string{"data": ""})

	task, err = vm.ResizeDisk(context.Background(), "scsi0", "40G")
	assert.Error(t, err)
	assert.Nil(t, task)
	assert.True(t, gock.IsDone())
}
