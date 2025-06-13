# Archive Documentation Index

This directory contains historical documentation that has been archived to maintain a clean working repository while preserving important context.

## 📁 Directory Structure

### `sprint-history/`
Contains documentation from completed sprints (1-7):
- Sprint planning documents
- Progress reports
- Completion summaries
- Performance benchmarking results

**Files**: 62 documents covering sprints 1-7

### `decisions-archive/`
Contains archived architectural and implementation decisions:
- Historical decision records from June 12, 2025
- Implementation strategy documents
- Technical resolution records

**Files**: 53 decision documents

### `notes-archive/`
Reserved for archived development notes and working documents.

## 🔍 Finding Archived Content

### Sprint-Related Documentation
```bash
# Find specific sprint documentation
find sprint-history/ -name "*sprint[1-7]*"

# Find performance benchmarking docs
find sprint-history/ -name "*performance*"
```

### Decision Records
```bash
# Find specific decision topics
grep -r "keyword" decisions-archive/

# List all decisions by date
ls -la decisions-archive/ | sort
```

## 📚 Current Active Documentation

For current development documentation, see:
- `../development/` - Active development notes and decisions
- `../` - Main documentation (API reference, guides, etc.)
- `../architecture/` - Current architecture documentation
- `../planning/` - Current planning and roadmap documents

## 🗂️ Archive Policy

Documents are archived when:
1. **Sprints are completed** - Sprint-specific documentation moves to `sprint-history/`
2. **Decisions are superseded** - Old decision records move to `decisions-archive/`
3. **Notes become historical** - Working notes that are no longer active move to `notes-archive/`

## 🔄 Restoration

If archived documentation needs to be restored to active status:
1. Move the file back to the appropriate active directory
2. Update any references or links
3. Consider if the content needs updating for current context 