#!/usr/bin/env bash
set -euo pipefail

cd "$SERVICE_PATH"
git config user.name "github-actions"
git config user.email "action@github.com"

last_commit_short=$(git rev-parse --short HEAD)
last_commit_message=$(git log -1 --pretty=format:"%s")
last_commit_author=$(git log -1 --pretty=format:"%an")

tag_name="v${SERVICE_VERSION}"

git fetch --tags origin || true

tag_exists_locally=$(git tag -l "$tag_name" | wc -l)
tag_exists_remotely=$(git ls-remote --tags origin "$tag_name" | wc -l)

# Tags are immutable, so a re-build of the same version must delete first.
if [ "$tag_exists_locally" -gt 0 ]; then
  echo "Tag $tag_name exists locally. Deleting..."
  git tag -d "$tag_name" || true
fi

if [ "$tag_exists_remotely" -gt 0 ]; then
  echo "Tag $tag_name exists remotely. Deleting..."
  git push origin --delete "$tag_name" || true
fi

cat > tag_message.txt << EOF
## Release ${SERVICE_NAME} v${SERVICE_VERSION}

Built from Branch: ${BRANCH}
Built on: $(TZ="Europe/Paris" date '+%Y-%m-%d %H:%M:%S %Z')
Author: $last_commit_author
Last Commit Message: $last_commit_message($last_commit_short)

---
*This tag was automatically created by GitHub Actions*
EOF

git tag -a "$tag_name" -F tag_message.txt
git push origin "$tag_name"
rm tag_message.txt
