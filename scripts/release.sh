#!/bin/sh
# Go 应用公开发版脚本：检查工作区/分支/版本占用、校验 CHANGELOG 与模块依赖、运行全量门禁、提交并推送发版分支与附注 tag。
set -eu

RELEASE_BRANCH="main"
RELEASE_KIND="app"
RELEASE_MODE="public"
BUILD_TAGS="with_v2ray_api,with_quic,with_utls,with_acme,with_clash_api,with_gvisor"

COMPLETED_STEPS="无"

usage() {
  echo "用法: $0 <x.y.z|x.y.z-<alpha|beta|rc|next>.<n>> [--dry-run]" >&2
}

fail() {
  echo "错误：$1" >&2
  echo "已完成步骤：${COMPLETED_STEPS}" >&2
  if [ -n "${2:-}" ]; then
    echo "继续方式：$2" >&2
  fi
  exit 1
}

mark_done() {
  if [ "$COMPLETED_STEPS" = "无" ]; then
    COMPLETED_STEPS="$1"
  else
    COMPLETED_STEPS="${COMPLETED_STEPS} -> $1"
  fi
}

cd "$(dirname "$0")/.."

VERSION=""
DRY_RUN=0

for arg in "$@"; do
  case "$arg" in
    --)
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      usage
      fail "未知选项：$arg" "按用法重新运行脚本"
      ;;
    *)
      if [ -n "$VERSION" ]; then
        usage
        fail "多余参数：$arg" "仅传入一个不带 v 前缀的版本号"
      fi
      VERSION="$arg"
      ;;
  esac
done

[ -n "$VERSION" ] || {
  usage
  fail "缺少目标版本号" "例如：$0 0.1.0 --dry-run"
}

case "$VERSION" in
  v*)
    fail "版本号不得带 v 前缀（收到：$VERSION）" "去掉 v 前缀后重跑，例如：$0 ${VERSION#v}"
    ;;
esac

printf '%s\n' "$VERSION" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(alpha|beta|rc|next)\.(0|[1-9][0-9]*))?$' \
  || fail "版本号格式非法：$VERSION（仅支持 x.y.z 或 x.y.z-<alpha|beta|rc|next>.<n>，禁止前导零与构建元数据）" "修正版本号后重跑"

TAG="v$VERSION"

extract_changelog_section() {
  [ -f CHANGELOG.md ] || return 1
  awk -v v="$VERSION" '
    $0 == "## " v || index($0, "## " v " ") == 1 || index($0, "## [" v "]") == 1 {
      found = 1
      print
      next
    }
    found && /^## / { exit }
    found { print }
  ' CHANGELOG.md
}

# 1. 工作区检查
OTHER_CHANGES=$(git status --porcelain | awk 'substr($0, 4) != "CHANGELOG.md" { print }')
[ -z "$OTHER_CHANGES" ] || fail "工作区存在 CHANGELOG.md 以外的未提交改动或未跟踪文件：
$OTHER_CHANGES" "先单独提交或清理上述文件后重跑"

STATUS_ALL=$(git status --porcelain)
RESUME=0
HEAD_SUBJECT=$(git log -1 --pretty=%s 2>/dev/null || true)

if [ -z "$STATUS_ALL" ]; then
  SECTION=$(extract_changelog_section || true)
  if [ -z "$SECTION" ]; then
    fail "工作区干净且 CHANGELOG.md 缺少版本 ${VERSION} 一节" "请先在 CHANGELOG.md 顶部编写 ## ${VERSION} (<YYYY-MM-DD>) 一节后再运行"
  fi
  case "$HEAD_SUBJECT" in
    "chore(release):"*)
      RESUME=1
      ;;
    *)
      fail "工作区干净且 CHANGELOG.md 已有 ${VERSION}，但 HEAD 提交不是 chore(release): 开头（当前：${HEAD_SUBJECT}）" "请检查提交历史或确认目标版本是否已被占用"
      ;;
  esac
fi

mark_done "1.工作区检查"

# 2. 分支检查
CURRENT_BRANCH=$(git symbolic-ref --quiet --short HEAD || true)
[ "$CURRENT_BRANCH" = "$RELEASE_BRANCH" ] \
  || fail "当前分支为 '${CURRENT_BRANCH:-游离HEAD}'，必须在 '${RELEASE_BRANCH}' 分支发版" "切换到 ${RELEASE_BRANCH} 分支并同步后重跑"

git fetch origin "$RELEASE_BRANCH" >/dev/null 2>&1 \
  || fail "无法执行 git fetch origin ${RELEASE_BRANCH}" "检查网络或 GitHub 凭证后重跑"

BEHIND_COUNT=$(git rev-list --count "HEAD..origin/${RELEASE_BRANCH}")
[ "$BEHIND_COUNT" -eq 0 ] \
  || fail "当前分支落后或分叉于 origin/${RELEASE_BRANCH}（落后 ${BEHIND_COUNT} 个提交）" "执行 git pull --rebase 后重跑"

mark_done "2.分支检查"

# 3. 版本占用检查
HEAD_SHA=$(git rev-parse HEAD)
LOCAL_TAG_SHA=$(git rev-parse -q --verify "refs/tags/${TAG}^{commit}" 2>/dev/null || true)

REMOTE_TAG_REFS=$(git ls-remote --tags origin "refs/tags/${TAG}" "refs/tags/${TAG}^{}")
REMOTE_TAG_SHA=$(printf '%s\n' "$REMOTE_TAG_REFS" | awk '$2 ~ /\^\{\}$/ { sha=$1 } END { if (sha != "") print sha }')
if [ -z "$REMOTE_TAG_SHA" ]; then
  REMOTE_TAG_SHA=$(printf '%s\n' "$REMOTE_TAG_REFS" | awk 'NR == 1 { print $1 }')
fi

if [ "$RESUME" -eq 0 ]; then
  [ -z "$LOCAL_TAG_SHA" ] || fail "本地已存在 tag ${TAG}（指向 ${LOCAL_TAG_SHA}）" "目标版本已被占用，请改用新版本号"
  [ -z "$REMOTE_TAG_SHA" ] || fail "远端已存在 tag ${TAG}（指向 ${REMOTE_TAG_SHA}）" "目标版本已被占用，请改用新版本号"
else
  if [ -n "$LOCAL_TAG_SHA" ] && [ "$LOCAL_TAG_SHA" != "$HEAD_SHA" ]; then
    fail "本地 tag ${TAG} 指向 ${LOCAL_TAG_SHA}，与当前 HEAD (${HEAD_SHA}) 不一致" "禁止删除或移动已存在 tag，请升级版本号"
  fi
  if [ -n "$REMOTE_TAG_SHA" ] && [ "$REMOTE_TAG_SHA" != "$HEAD_SHA" ]; then
    fail "远端 tag ${TAG} 指向 ${REMOTE_TAG_SHA}，与当前 HEAD (${HEAD_SHA}) 不一致" "禁止删除或移动已推送 tag，请升级版本号"
  fi
fi

mark_done "3.版本占用检查"

# 4. 内容与模块检查（续发也执行）
[ -f CHANGELOG.md ] || fail "仓库根目录缺少 CHANGELOG.md" "先创建 CHANGELOG.md 并写入 ## ${VERSION} 一节"

SECTION=$(extract_changelog_section || true)
[ -n "$SECTION" ] || fail "CHANGELOG.md 缺少版本 ${VERSION} 一节（标题需为 ## ${VERSION} 或 ## ${VERSION} (<YYYY-MM-DD>)）" "补全 CHANGELOG.md 后重跑"

CHANGELOG_TITLE=$(printf '%s\n' "$SECTION" | sed -n '1p')
CHANGELOG_BODY=$(printf '%s\n' "$SECTION" | sed '1d')
printf '%s\n' "$CHANGELOG_BODY" | grep -q '[^[:space:]]' \
  || fail "CHANGELOG.md 中 ${CHANGELOG_TITLE} 的正文为空" "请填写面向使用者的变更条目后重跑"

go mod tidy || fail "go mod tidy 执行失败" "修复 go.mod 依赖问题后重跑"
if ! git diff --quiet -- go.mod go.sum; then
  git checkout -- go.mod go.sum
  fail "go mod tidy 产生了未提交的 go.mod / go.sum 变更（已自动还原）" "请先运行 go mod tidy 并单独提交后再发版"
fi

if [ -d vendor ]; then
  go mod vendor || fail "go mod vendor 执行失败" "修复 vendor 问题后重跑"
  VENDOR_DIFF=$(git status --porcelain -- vendor)
  if [ -n "$VENDOR_DIFF" ]; then
    git checkout -- vendor
    fail "go mod vendor 产生了未提交变更（已自动还原）" "请先提交 go mod vendor 结果后再发版"
  fi
fi

go test -tags "$BUILD_TAGS" ./... || fail "Go 测试失败（go test -tags ${BUILD_TAGS} ./...）" "修复失败测试并提交后重跑"

if [ -f web/package.json ]; then
  command -v bun >/dev/null 2>&1 || fail "检测到 web/package.json 但未安装 bun" "安装 bun 后重跑"
  (cd web && bun install --frozen-lockfile && bun run build) || fail "前端安装校验或构建失败" "修复前端问题并提交后重跑"
fi

POST_CHECK_CHANGES=$(git status --porcelain | awk 'substr($0, 4) != "CHANGELOG.md" { print }')
[ -z "$POST_CHECK_CHANGES" ] || fail "门禁执行后工作区出现了预期之外的改动：
$POST_CHECK_CHANGES" "检查构建产物或 gitignore 配置后清理重跑"

mark_done "4.内容与模块检查"

# 5. --dry-run 在此结束
if [ "$DRY_RUN" -eq 1 ]; then
  echo "=== 发版演练通过（--dry-run） ==="
  echo "release_kind : ${RELEASE_KIND}"
  echo "release_mode : ${RELEASE_MODE}"
  echo "release_branch: ${RELEASE_BRANCH}"
  echo "version      : ${VERSION}"
  echo "tag          : ${TAG}"
  echo "changelog    : ${CHANGELOG_TITLE}"
  echo "checks       : go mod tidy 无 diff, go test (-tags ${BUILD_TAGS}), web frozen build 均通过"
  if [ "$RESUME" -eq 1 ]; then
    echo "mode         : 续发（HEAD=${HEAD_SHA}）"
    if [ -n "$REMOTE_TAG_SHA" ]; then
      echo "next_steps   : 远端 tag ${TAG} 已指向 HEAD，正式执行将跳过提交/打 tag，直接检查并触发或重跑 release.yml"
    else
      echo "next_steps   : 正式执行将检查并推送 ${RELEASE_BRANCH}、补齐并推送 tag ${TAG}"
    fi
  else
    echo "mode         : 首次提交并发布"
    echo "next_steps   : git add CHANGELOG.md -> git commit 'chore(release): 发布 ${TAG}' -> git push origin HEAD:${RELEASE_BRANCH} -> git tag -a ${TAG} -> git push origin ${TAG}"
  fi
  exit 0
fi

# 6. 提交并推送发版分支
if [ "$RESUME" -eq 0 ]; then
  git add CHANGELOG.md
  git commit -m "chore(release): 发布 ${TAG}" || fail "提交 CHANGELOG.md 失败" "检查 git 状态后重跑"
  HEAD_SHA=$(git rev-parse HEAD)
fi

REMOTE_BRANCH_SHA=$(git ls-remote origin "refs/heads/${RELEASE_BRANCH}" | awk 'NR == 1 { print $1 }')
if [ "$REMOTE_BRANCH_SHA" != "$HEAD_SHA" ]; then
  git push origin "HEAD:${RELEASE_BRANCH}" || fail "推送 ${RELEASE_BRANCH} 分支失败" "修复远端同步或权限问题后直接重跑（脚本将自动续发）"
  REMOTE_BRANCH_SHA=$(git ls-remote origin "refs/heads/${RELEASE_BRANCH}" | awk 'NR == 1 { print $1 }')
  [ "$REMOTE_BRANCH_SHA" = "$HEAD_SHA" ] || fail "推送后远端 ${RELEASE_BRANCH} (${REMOTE_BRANCH_SHA}) 与 HEAD (${HEAD_SHA}) 不一致" "检查远端分支后重跑"
fi

mark_done "5.提交并推送分支"

# 7. 打附注 tag 并推送
TAG_ALREADY_REMOTE=0
if [ -n "$REMOTE_TAG_SHA" ]; then
  TAG_ALREADY_REMOTE=1
fi

if [ -z "$LOCAL_TAG_SHA" ]; then
  git tag -a "$TAG" -m "$TAG" || fail "创建本地附注 tag ${TAG} 失败" "排查后直接重跑（脚本将自动续发）"
fi

if [ "$TAG_ALREADY_REMOTE" -eq 0 ]; then
  git push origin "$TAG" || fail "推送 tag ${TAG} 失败" "不要更换版本号，修复网络或权限后直接重跑续发"
  PUSHED_TAG_REFS=$(git ls-remote --tags origin "refs/tags/${TAG}" "refs/tags/${TAG}^{}")
  PUSHED_TAG_SHA=$(printf '%s\n' "$PUSHED_TAG_REFS" | awk '$2 ~ /\^\{\}$/ { sha=$1 } END { if (sha != "") print sha }')
  [ "$PUSHED_TAG_SHA" = "$HEAD_SHA" ] || fail "远端 tag 核对失败：refs/tags/${TAG}^{}='${PUSHED_TAG_SHA}'，期望='${HEAD_SHA}'" "检查远端 tag 状态"
fi

mark_done "6.创建并推送tag"

# 8. 应用 + 公开发版收尾（CI 负责构建与创建 GitHub Release）
if [ "$RESUME" -eq 1 ] && [ "$TAG_ALREADY_REMOTE" -eq 1 ]; then
  command -v gh >/dev/null 2>&1 || fail "续发且远端 tag 已存在，需要 gh CLI 查询/重跑 release.yml" "安装并登录 gh 后重跑"
  RUNS_JSON=$(gh run list --workflow release.yml --commit "$HEAD_SHA" --json databaseId,status,conclusion) \
    || fail "查询 release.yml 运行记录失败" "检查 gh 登录状态后重跑"
  if [ "$RUNS_JSON" = "[]" ] || [ -z "$RUNS_JSON" ]; then
    echo "未检测到 commit ${HEAD_SHA} 的 release.yml 运行记录，手动派发（--ref ${TAG}）..."
    gh workflow run release.yml --ref "$TAG" || fail "触发 release.yml 失败" "检查 GitHub Actions 权限后重试"
  else
    FAILED_RUN_ID=$(printf '%s\n' "$RUNS_JSON" | awk '
      /"databaseId"/ { gsub(/[^0-9]/, "", $0); id = $0 }
      /"conclusion":[[:space:]]*"(failure|cancelled|timed_out|startup_failure)"/ { if (id != "") { print id; exit } }
    ')
    if [ -n "$FAILED_RUN_ID" ]; then
      echo "检测到失败的 release.yml run (${FAILED_RUN_ID})，执行 --failed 重跑..."
      gh run rerun "$FAILED_RUN_ID" --failed || fail "重跑 run ${FAILED_RUN_ID} 失败" "到 GitHub Actions 页面排查原因"
    else
      echo "已存在正在运行或已成功的 release.yml 记录，跳过触发。"
    fi
  fi
fi

mark_done "7.触发公开发版收尾"

echo "=== 公开发版已触发（由 GitHub Actions 构建二进制并创建 Release） ==="
echo "release_kind : ${RELEASE_KIND}"
echo "release_mode : ${RELEASE_MODE}"
echo "version      : ${VERSION}"
echo "tag          : ${TAG}"
echo "commit       : ${HEAD_SHA}"
echo "下一步跟踪与验证："
echo "  gh run list --workflow release.yml --commit \"${HEAD_SHA}\" --limit 1"
echo "  gh run watch <run-id> --exit-status"
echo "  gh release view \"${TAG}\" --json tagName,assets,isPrerelease"
