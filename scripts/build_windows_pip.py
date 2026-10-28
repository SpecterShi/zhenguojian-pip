import argparse
import base64
import hashlib
import re
from pathlib import Path
import shutil
import subprocess


parser = argparse.ArgumentParser()
parser.add_argument('--base-package', required=True, type=Path)
parser.add_argument('--flutter', required=True, type=Path)
parser.add_argument('--native-core', type=Path, help='Use a freshly built native core from this source.')
options = parser.parse_args()
root = Path.cwd()
if not (root / 'pubspec.yaml').is_file():
    raise SystemExit('Run from the project root; use an ASCII directory alias on Windows.')
baseline = options.base_package.resolve()
flutter = options.flutter.resolve()
engine = flutter.parent / 'cache/artifacts/engine/windows-x64-release/flutter_windows.dll'


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


if sha256(baseline / 'flutter_windows.dll') != sha256(engine):
    raise SystemExit('Flutter engine does not match baseline; use Flutter 3.47.4.')
if not options.native_core and sha256(baseline / 'duanju_core.dll') != 'd8c18edce72c8b479102b5dca4ea8420c22a41f3f624d8617ce9c9d5890229b6':
    raise SystemExit('Expected the verified build-9 native core.')
if options.native_core and not options.native_core.is_file():
    raise SystemExit('The rebuilt native core does not exist.')
bundle = root / 'build/windows/x64/runner/Release'
if bundle.exists():
    raise SystemExit('Release bundle already exists; preserve it before rebuilding.')
defines = base64.b64encode(b'ALL_SOURCES=true').decode('ascii')
version = re.search(r'^version:\s*(\S+)', (root / 'pubspec.yaml').read_text(encoding='utf-8'), re.MULTILINE).group(1)
build_name, build_number = version.split('+')
subprocess.run([
    str(flutter), 'assemble', '--no-version-check', '--output=build',
    '-dTargetPlatform=windows-x64', '-dTrackWidgetCreation=false',
    '-dBuildMode=release', '-dTargetFile=lib/main.dart', '-dTreeShakeIcons=true',
    '-dDartObfuscation=false', '-dBuildName=' + build_name, '-dBuildNumber=' + build_number,
    '--DartDefines=' + defines, 'release_bundle_windows-x64_assets',
], check=True, cwd=root)
shutil.copytree(baseline, bundle)
assets = bundle / 'data/flutter_assets'
if not assets.resolve().is_relative_to(bundle.resolve()):
    raise SystemExit('Invalid asset destination.')
shutil.rmtree(assets)
shutil.copytree(root / 'build/flutter_assets', assets)
shutil.copy2(root / 'build/windows/app.so', bundle / 'data/app.so')
if options.native_core:
    shutil.copy2(options.native_core, bundle / 'duanju_core.dll')
print('Built Windows PiP application:', bundle)
print('Reused unchanged build-9 runner and player libraries; rebuilt core:', bool(options.native_core))
