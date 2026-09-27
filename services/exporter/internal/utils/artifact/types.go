package artifact

type Stage int

const (
	StageNone Stage = iota
	StageTempCreate
	StageTempWrite
	StageTempClose
	StageRename
)
