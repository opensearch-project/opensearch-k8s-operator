# Security Config Merge - Implementation Details

This document provides technical details about the security config merge feature implementation.

## Problem Statement

When applying custom security configurations via the `securityConfigSecret`, the operator was completely replacing the security configuration in OpenSearch. This caused **all existing internal users** to be deleted, including:
- Users created through OpenSearchUser CRDs
- Users manually created in OpenSearch
- Any users not present in the custom security config

## Solution Overview

The operator now supports two modes:

1. **Merge Mode (Default)**: Preserves existing internal users by merging them with custom config
2. **Overwrite Mode**: Legacy behavior that replaces all config (enabled via annotation)

## Technical Implementation

### 1. Annotation-Based Control

**Helper Function** (`pkg/helpers/helpers.go`):
```go
func ShouldMergeSecurityConfig(cluster *opensearchv1.OpenSearchCluster) bool {
    if cluster.Annotations == nil {
        return true // Default to merge mode
    }

    value, exists := cluster.Annotations["opensearch.org/securityconfig-overwrite"]
    if !exists {
        return true // Merge mode if annotation absent
    }

    return value != "true" // Only overwrite if explicitly set to "true"
}
```

### 2. Backup Current Configuration

**Backup Command** (`pkg/reconcilers/securityconfig.go`):
```bash
BACKUP_DIR=/tmp/security-backup;
mkdir -p $BACKUP_DIR;
echo "Backing up current security configuration...";
$ADMIN -backup $BACKUP_DIR -cacert <ca> -cert <cert> -key <key> \
  -icl -nhnv -h <host> -p <port>;
```

Uses the `-backup` option of `securityadmin.sh` to retrieve the current security configuration from OpenSearch.

### 3. Merge Internal Users

**Merge Command** (`pkg/reconcilers/securityconfig.go`):
```bash
YQ_CMD=/tools/yq;
BACKUP_USERS=/tmp/security-backup/internal_users.yml;
CUSTOM_USERS=<securityconfig-path>/internal_users.yml;
MERGED_USERS=/tmp/merged_internal_users.yml;

# Merge: custom config overrides backup
$YQ_CMD eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
  $BACKUP_USERS $CUSTOM_USERS > $MERGED_USERS;
```

Key points:
- Uses `yq` YAML processor for merging
- Custom config takes precedence (fileIndex == 1)
- Output written to **writable temp location** (`/tmp/merged_internal_users.yml`)
- Original mounted files remain read-only

### 4. Apply Configuration

**Modified Application Logic**:
```go
for _, k := range keys {
    var filePath string

    // Use merged file for internal_users.yml in merge mode
    if shouldMerge && k == "internal_users.yml" {
        filePath = "/tmp/merged_internal_users.yml"
    } else {
        filePath = fmt.Sprintf("%s/%s", securityconfigPath, k)
    }

    // Apply the file
    securityadmin.sh -f <filePath> -t <fileType> ...
}
```

Each security config file is applied individually:
- `internal_users.yml`: Uses merged file from `/tmp/` (merge mode) or original (overwrite mode)
- Other files (`roles.yml`, `roles_mapping.yml`, etc.): Always use original mounted files

### 5. Tool Installation

**Init Container** (`pkg/builders/cluster.go`):
```go
InitContainers: []corev1.Container{{
    Name:  "install-yq",
    Image: <init-helper-image>,
    Command: []string{"/bin/sh", "-c"},
    Args: []string{`
        YQ_VERSION=v4.35.1
        wget -qO /tools/yq https://github.com/mikefarah/yq/.../yq_linux_amd64
        chmod +x /tools/yq
    `},
    VolumeMounts: []corev1.VolumeMount{{
        Name:      "tools",
        MountPath: "/tools",
    }},
}},
```

Downloads and installs `yq` binary to a shared volume before the main container starts.

## Workflow Comparison

### Merge Mode (Default)
```
1. Wait for cluster ready
2. Backup current config → /tmp/security-backup/
3. Merge internal_users.yml → /tmp/merged_internal_users.yml
4. Apply merged internal_users.yml
5. Apply other config files (roles, mappings, etc.)
```

### Overwrite Mode (Annotation Set)
```
1. Wait for cluster ready
2. Apply internal_users.yml directly
3. Apply other config files (roles, mappings, etc.)
```

## Key Design Decisions

### Why Write Merged File to /tmp?

**Problem**: Security config files are mounted from Kubernetes secrets as **read-only volumes**.

**Attempted Solution**: Tried to overwrite the mounted file after merge.

**Error**:
```
cp: cannot create regular file '/usr/share/opensearch/config/opensearch-security/internal_users.yml':
Read-only file system
```

**Final Solution**:
- Write merged file to `/tmp/merged_internal_users.yml` (writable)
- Reference the temp file when applying via `securityadmin.sh -f`
- Other files remain in their original mounted locations

### Why yq Instead of Python?

**Advantages of yq**:
- ✅ Single binary, easy to install via wget
- ✅ Purpose-built for YAML operations
- ✅ Simple merge syntax: `eval-all 'select(fileIndex == 0) * select(fileIndex == 1)'`
- ✅ No dependencies to install

**Python Alternative Would Require**:
- Installing PyYAML module
- More complex script
- Larger init container

### Why Init Container?

**Why Not Pre-install in OpenSearch Image?**
- ❌ Would require custom image builds
- ❌ Would need to maintain fork of opensearch image
- ❌ Harder for users to adopt

**Init Container Approach**:
- ✅ Works with any OpenSearch image
- ✅ Downloads yq on-demand
- ✅ Transparent to users
- ✅ Requires internet access (or pre-cached yq in init helper image)

## Merge Behavior

### Merge Priority

When merging `internal_users.yml`:

```yaml
# Backup (from OpenSearch)
admin:
  hash: "$2a$12$..."
  reserved: true

kibanaserver:
  hash: "$2a$12$..."
  reserved: true

existing_user:
  hash: "$2a$12$..."
  backend_roles: ["role1"]

# Custom (from secret)
custom_user:
  hash: "$2a$12$..."
  backend_roles: ["role2"]

existing_user:
  hash: "$2a$12$NEW_HASH"
  backend_roles: ["role3"]

# Result after merge
admin:
  hash: "$2a$12$..."
  reserved: true

kibanaserver:
  hash: "$2a$12$..."
  reserved: true

existing_user:              # Custom overrides backup
  hash: "$2a$12$NEW_HASH"
  backend_roles: ["role3"]

custom_user:                # Custom adds new user
  hash: "$2a$12$..."
  backend_roles: ["role2"]
```

### Edge Cases Handled

1. **No custom internal_users.yml**: Uses backup as-is
2. **No backup (first run)**: Uses custom config as-is
3. **Empty custom file**: Preserves all existing users
4. **User exists in both**: Custom definition takes precedence

## Testing Considerations

### Unit Tests

Test the `ShouldMergeSecurityConfig()` helper:
- No annotation → returns `true`
- Annotation absent → returns `true`
- Annotation = "false" → returns `true`
- Annotation = "true" → returns `false`

### Integration Tests

Test scenarios:
1. Apply custom config without annotation (merge mode)
   - Verify existing users preserved
   - Verify custom users added
   - Verify user conflicts resolved (custom wins)

2. Apply custom config with overwrite annotation
   - Verify existing users deleted
   - Verify only custom users present

3. Toggle between modes
   - Start with merge mode
   - Switch to overwrite mode
   - Switch back to merge mode

### Job Logs to Check

Look for these log messages:
```
"Security config merge mode enabled - will preserve existing internal users"
"Backing up current security configuration..."
"Backup completed successfully"
"Merging internal_users.yml (custom config takes precedence)..."
"Merge completed successfully - merged file at /tmp/merged_internal_users.yml"
```

Or:
```
"Security config overwrite mode enabled - will replace all internal users"
```

## Troubleshooting Guide

### Common Issues

**Issue 1: yq download fails**
```
ERROR: yq not found at /tools/yq
```
**Solution**:
- Check init container logs for wget errors
- Ensure pods have internet access
- Consider pre-installing yq in init helper image

**Issue 2: Backup fails**
```
ERROR: Failed to backup security configuration after 20 attempts
```
**Solution**:
- Verify admin certificate is valid
- Check cluster is reachable from security job pod
- Verify TLS configuration

**Issue 3: Merge fails**
```
ERROR: Failed to merge internal_users.yml
```
**Solution**:
- Check custom `internal_users.yml` is valid YAML
- Verify file exists in secret
- Check yq version compatibility

**Issue 4: Read-only filesystem**
```
cp: cannot create regular file '...': Read-only file system
```
**Solution**:
- This should be fixed by writing to `/tmp/` instead
- If still occurs, check the implementation uses temp location

## Performance Considerations

### Resource Usage

**Additional Resources Required**:
- Init container: ~50MB memory, minimal CPU
- yq binary: ~10MB disk space
- Backup data: ~1MB temp storage
- Merge operation: Negligible CPU/memory

**Job Execution Time**:
- Without merge: ~30-60 seconds
- With merge: +10-20 seconds for backup/merge
- Total: ~40-80 seconds

### Optimization Opportunities

1. **Cache yq binary**: Pre-install in init helper image to skip download
2. **Skip backup**: If internal_users.yml not in custom config, skip backup
3. **Parallel operations**: Could parallelize backup and custom file reads

## Security Considerations

### Attack Vectors

1. **Malicious yq binary**: Downloaded from GitHub releases
   - Mitigation: Use checksums (future enhancement)
   - Alternative: Pre-install verified binary

2. **Temp file exposure**: Merged file in `/tmp/`
   - Risk: Low (pod-local filesystem)
   - Mitigation: Files cleaned up when pod terminates

3. **Backup data exposure**: Contains password hashes
   - Risk: Low (pod-local, temporary)
   - Mitigation: Stored in `/tmp/`, cleared on completion

### Best Practices

1. Always use TLS for admin certificate
2. Rotate admin credentials regularly
3. Use strong bcrypt hashes for user passwords
4. Monitor security job logs for failures
5. Keep custom security configs in version control

## Future Enhancements

Potential improvements:

1. **Selective file merging**: Support merging other files (roles, mappings)
2. **Conflict resolution strategies**: Allow user-defined merge behavior
3. **Dry-run mode**: Preview merge results before applying
4. **Backup retention**: Keep previous backups for rollback
5. **Checksum verification**: Verify yq binary integrity
6. **Offline mode**: Support air-gapped environments