package util

import (
	"errors"
	"fmt"
	"strings"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CreateNFSBackedPersistentVolume creates a persistent volume backed by a pod
// running nfs
func CreateNFSBackedPersistentVolume(name, capacity, server string, number int) *kapiv1.PersistentVolume {
	e2e.Logf("Creating persistent volume %s", name)
	return &kapiv1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.PersistentVolumeSpec{
			PersistentVolumeSource: kapiv1.PersistentVolumeSource{
				NFS: &kapiv1.NFSVolumeSource{
					Server:   server,
					Path:     fmt.Sprintf("/exports/data-%d", number),
					ReadOnly: false,
				},
			},
			PersistentVolumeReclaimPolicy: kapiv1.PersistentVolumeReclaimDelete,
			Capacity: kapiv1.ResourceList{
				kapiv1.ResourceStorage: resource.MustParse(capacity),
			},
			AccessModes: []kapiv1.PersistentVolumeAccessMode{
				kapiv1.ReadWriteMany,
				kapiv1.ReadWriteOnce,
				kapiv1.ReadOnlyMany,
			},
		},
	}
}

// SetupNFSBackedPersistentVolumes sets up an NFS backed persistent volume
// Currently, the image that is used exports 10 nfs shares, so the maximum
// integer for count would be 10
func SetupNFSBackedPersistentVolumes(oc *CLI, capacity string, count int) (volumes []*kapiv1.PersistentVolume, err error) {
	e2e.Logf("Setting up nfs backed persistent volume(s)")

	maxVolumes := 10
	if count > maxVolumes {
		return nil, errors.New(fmt.Sprintf("SetupNFSBackedPersistentVolumes only supports a maximum of %d volumes, you specified %d", maxVolumes, count))
	}

	c := oc.AdminKubeClient().Core().PersistentVolumes()
	prefix := oc.Namespace()

	_, svc, err := SetupNFSServer(oc, capacity)
	if err != nil {
		return nil, err
	}
	server, err := oc.Run("get").Args("service", svc.Name, "--template", "{{.spec.clusterIP}}").Output()
	if err != nil {
		return nil, err
	}

	for i := 0; i < count; i++ {
		pv, err := c.Create(CreateNFSBackedPersistentVolume(fmt.Sprintf("%s%s-%d", pvPrefix, prefix, i), capacity, server, i))
		if err != nil {
			e2e.Logf("Error creating persistent volume %v\n", err)
			return nil, err
		}
		volumes = append(volumes, pv)
	}
	return volumes, nil
}

// RemoveNFSBackedPersistentVolumes removes persistent volumes created by SetupNFSBackedPersistentVolume
func RemoveNFSBackedPersistentVolumes(oc *CLI) error {
	c := oc.AdminKubeClient().Core().PersistentVolumes()
	prefix := oc.Namespace()

	pvs, err := c.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pv := range pvs.Items {
		if !strings.HasPrefix(pv.Name, fmt.Sprintf("%s%s", pvPrefix, prefix)) {
			e2e.Logf("Skipping removing persistent volume: %s", pv.Name)
			continue
		}

		if err = c.Delete(pv.Name, nil); err != nil {
			e2e.Logf("WARNING: couldn't remove PV %s: %v\n", pv.Name, err)
			continue
		}
		e2e.Logf("Removed persistent volume: %s", pv.Name)
	}

	if err = RemoveNFSServer(oc); err != nil {
		return err
	}

	return nil
}
