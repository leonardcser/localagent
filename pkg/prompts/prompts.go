package prompts

import _ "embed"

//go:embed system-identity.txt
var SystemIdentity string

//go:embed tools-section.txt
var ToolsSection string

//go:embed skills-section.txt
var SkillsSection string

//go:embed memory-flush-system.txt
var MemoryFlushSystem string

//go:embed memory-flush-user.txt
var MemoryFlushUser string

//go:embed summarize-batch.txt
var SummarizeBatch string

//go:embed summarize-merge.txt
var SummarizeMerge string

//go:embed subagent-async.txt
var SubagentAsync string

//go:embed subagent-sync.txt
var SubagentSync string

//go:embed heartbeat.txt
var Heartbeat string

//go:embed heartbeat-template.txt
var HeartbeatTemplate string
