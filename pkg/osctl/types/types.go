package types

// Template
//
//	dataDisk:
//	- /data
//
// sysDisk:
//
//	/: 100G
//	/boot: 2G
//	/data: rest
//	NOSWAP: "true"
//
// raid:
//   - diskSize: 1.2T
//     raidCount: 1
//     raidLevel: R1
//     raidMembers: 2
type Template struct {
	SysDisk  map[string]string `json:"sysDisk"`
	DataDisk []string          `json:"dataDisk"`
	Raids    []Raid            `json:"raids"`
}
type Raid struct {
	RaidLevel   string `json:"raidLevel,omitempty"`
	RaidCount   int64  `json:"raidCount,omitempty"`
	RaidMembers int64  `json:"raidMembers,omitempty"`
	DiskSize    string `json:"diskSize,omitempty"`
}
