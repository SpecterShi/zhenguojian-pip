"""构建红果鉴 / 真果鉴 macOS 通用版（Apple Silicon + Intel）。

必须在一台 Mac（macOS 12+，已安装 Xcode 与命令行工具）上运行；
Windows / Linux 无法交叉编译 Flutter macOS 应用。

用法：
    python3 scripts/build_macos.py                # 红果鉴（仅红果站源）
    python3 scripts/build_macos.py --all-sources  # 真果鉴（全部站源）
    python3 scripts/build_macos.py --all-sources --cn-mirrors
    python3 scripts/build_macos.py --core-only    # 只编译 Go 核心（调试用）

产物：dist/macos/<slug>-<version>-macos-universal.zip（含 .app，未签名）
"""

import argparse
import hashlib
import os
import platform
import re
import shlex
import shutil
import subprocess
from pathlib import Path

from app_build import BuildVariant, add_variant_argument

root = Path(__file__).resolve().parents[1]

# pubspec.yaml 中仅服务 Windows 构建的覆盖项；macOS 构建前需临时移除。
WINDOWS_OVERRIDE_BLOCK = """dependency_overrides:
  media_kit_libs_windows_video:
    path: packages/media_kit_libs_windows_video
"""


def run(arguments, **kwargs):
    subprocess.run(arguments, cwd=kwargs.pop('cwd', root), check=True, **kwargs)


def configure_branding(variant):
    """按变体写窗口产品名与 Dock 显示名。"""
    info = root / 'macos' / 'Runner' / 'Configs' / 'AppInfo.xcconfig'
    text = info.read_text(encoding='utf-8')
    text = re.sub(r'^PRODUCT_NAME = .*$', f'PRODUCT_NAME = {variant.slug}', text, flags=re.M)
    info.write_text(text, encoding='utf-8')
    plist = root / 'macos' / 'Runner' / 'Info.plist'
    text = plist.read_text(encoding='utf-8')
    text = re.sub(
        r'(<key>CFBundleDisplayName</key>\s*<string>).*?(</string>)',
        rf'\g<1>{variant.name}\g<2>',
        text,
    )
    plist.write_text(text, encoding='utf-8')


def build_core(variant):
    """编译 Go 核心为 macOS 通用静态库（arm64 + amd64），生成 DuanjuCore.xcframework。"""
    go = shutil.which('go')
    if not go:
        raise SystemExit('请安装 Go 1.24.1 或更新版本（brew install go 或 https://go.dev/dl/）。')
    environment = os.environ.copy()
    environment.setdefault('GOPROXY', 'https://goproxy.cn,direct')
    environment.setdefault('GOSUMDB', 'off')
    environment['CGO_ENABLED'] = '1'
    sdk_path = subprocess.check_output(['xcrun', '--sdk', 'macosx', '--show-sdk-path'], text=True).strip()
    compiler = subprocess.check_output(['xcrun', '--sdk', 'macosx', '--find', 'clang'], text=True).strip()
    libraries = []
    for architecture in ['arm64', 'amd64']:
        # Apple Silicon 上交叉编译 Intel slice 时 clang 需要显式 -arch（否则编出 host 架构导致链接失败）。
        clang_arch = 'arm64' if architecture == 'arm64' else 'x86_64'
        flags = shlex.join(['-isysroot', sdk_path, '-mmacosx-version-min=12.0', '-arch', clang_arch])
        directory = root / 'native' / 'build' / 'macos' / architecture
        headers = directory / 'Headers'
        headers.mkdir(parents=True, exist_ok=True)
        output = directory / 'libDuanjuCore.a'
        build_env = environment | {
            'GOOS': 'darwin', 'GOARCH': architecture, 'CC': compiler,
            'CGO_CFLAGS': flags, 'CGO_LDFLAGS': flags,
        }
        print(f'Building native/build/macos/{architecture}/libDuanjuCore.a', flush=True)
        run([go, 'build', '-trimpath', '-buildmode=c-archive', '-ldflags=' + variant.linker_flags,
             '-o', str(output), './bridge'], cwd=root / 'native', env=build_env)
        shutil.copy2(output.with_suffix('.h'), headers / 'DuanjuCore.h')
        libraries.append((architecture, output, headers))
    universal = root / 'native' / 'build' / 'macos' / 'universal'
    universal.mkdir(parents=True, exist_ok=True)
    archive = universal / 'libDuanjuCore.a'
    run(['lipo', '-create', str(libraries[0][1]), str(libraries[1][1]), '-output', str(archive)])
    framework = root / 'macos' / 'DuanjuCore' / 'DuanjuCore.xcframework'
    if framework.exists():
        shutil.rmtree(framework)
    run(['xcodebuild', '-create-xcframework',
         '-library', str(archive), '-headers', str(libraries[0][2]),
         '-output', str(framework)])
    print(f'DuanjuCore.xcframework 就绪：{framework}', flush=True)


def main():
    if platform.system() != 'Darwin':
        raise SystemExit('macOS 构建需要一台 Mac（macOS 12+，已安装完整 Xcode）；'
                         'Windows / Linux 无法交叉编译 Flutter macOS 应用。')
    parser = argparse.ArgumentParser(description='构建红果鉴 / 真果鉴 macOS 通用版')
    parser.add_argument('--core-only', action='store_true', help='只编译 Go 核心，不构建应用')
    parser.add_argument('--cn-mirrors', action='store_true', help='使用 Flutter / pub 中国镜像')
    add_variant_argument(parser)
    options = parser.parse_args()
    variant = BuildVariant(options.all_sources)
    configure_branding(variant)
    build_core(variant)
    if options.core_only:
        return
    flutter = shutil.which('flutter')
    if not flutter or not shutil.which('pod'):
        raise SystemExit('请安装 Flutter（3.47.4 或更新，见 README 开发与构建一节）和 CocoaPods'
                         '（brew install cocoapods 或 sudo gem install cocoapods）。')
    environment = os.environ.copy()
    if options.cn_mirrors:
        environment.setdefault('PUB_HOSTED_URL', 'https://pub.flutter-io.cn')
        environment.setdefault('FLUTTER_STORAGE_BASE_URL', 'https://storage.flutter-io.cn')
    pubspec = root / 'pubspec.yaml'
    original = pubspec.read_text(encoding='utf-8')
    stripped = original.replace(WINDOWS_OVERRIDE_BLOCK, '')
    changed = stripped != original
    if changed:
        pubspec.write_text(stripped, encoding='utf-8')
    try:
        run([flutter, 'pub', 'get'], env=environment)
        run(['pod', 'install'], cwd=root / 'macos', env=environment)
        run([flutter, 'build', 'macos', '--release', '--no-pub', *variant.flutter_arguments],
            env=environment)
    finally:
        if changed:
            pubspec.write_text(original, encoding='utf-8')
    application = root / 'build' / 'macos' / 'Build' / 'Products' / 'Release' / f'{variant.slug}.app'
    if not application.is_dir():
        raise SystemExit('未找到应用产物：' + str(application))
    executable = application / 'Contents' / 'MacOS' / variant.slug
    symbols = subprocess.check_output(['nm', '-gU', str(executable)], text=True)
    for symbol in ['_DuanjuRequest', '_DuanjuFree']:
        if symbol not in symbols:
            raise SystemExit('macOS 包缺少 FFI 入口：' + symbol)
    print('FFI 入口校验通过（DuanjuRequest / DuanjuFree 已链接）', flush=True)
    output = root / 'dist' / 'macos'
    output.mkdir(parents=True, exist_ok=True)
    version = re.search(r'^version:\s*(\S+)', (root / 'pubspec.yaml').read_text(encoding='utf-8'), re.MULTILINE).group(1)
    destination = output / f'{variant.slug}-{version}-macos-universal.zip'
    if destination.exists():
        destination.unlink()
    run(['ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', str(application), str(destination)])
    digest = hashlib.sha256()
    with destination.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    (output / 'SHA256SUMS.txt').write_text(f'{digest.hexdigest()}  {destination.name}\n', encoding='ascii')
    print(destination)


if __name__ == '__main__':
    main()
