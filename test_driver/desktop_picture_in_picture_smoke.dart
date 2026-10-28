import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;

import 'package:duanju_app/danmaku_models.dart';
import 'package:duanju_app/danmaku_overlay.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/models.dart';
import 'package:duanju_app/player_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:media_kit/media_kit.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:window_manager/window_manager.dart';

import '../test/fixtures.dart';

class LocalVideoRepository extends FixtureRepository {
  LocalVideoRepository(this.path);
  final String path;

  @override
  Future<PlaybackPlan> resolve(
    Drama drama,
    Episode episode, {
    int quality = 0,
  }) async => PlaybackPlan(
    url: path,
    session: 'synthetic-video',
    danmakuId: '${episode.number}',
  );

  @override
  Future<DanmakuPage> danmaku(
    PlaybackPlan plan, {
    required int startMs,
    required int durationMs,
  }) async {
    final end = math.min(startMs + danmakuWindowMs, durationMs);
    return DanmakuPage(
      episodeId: plan.danmakuId,
      startMs: startMs,
      nextMs: end,
      items: [
        for (var time = startMs; time < end; time += 2000)
          DanmakuItem(
            id: '${plan.danmakuId}-$time',
            text: '小窗弹幕测试 $time',
            timeMs: time,
          ),
      ],
    );
  }
}

Future<void> main(List<String> arguments) async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();
  MediaKit.ensureInitialized();
  final report = File(arguments[0]);
  final steps = <String, Object?>{};
  Player? player;
  LocalStore? store;
  final errors = <String>[];
  FlutterError.onError = (details) => errors.add(details.exceptionAsString());

  void check(bool condition, String message) {
    if (!condition) throw StateError(message);
  }

  Future<void> waitFor(bool Function() ready) async {
    final timer = Stopwatch()..start();
    while (!ready()) {
      if (timer.elapsed > const Duration(seconds: 25)) {
        throw TimeoutException('Timed out waiting for playback or window');
      }
      await Future<void>.delayed(const Duration(milliseconds: 100));
    }
  }

  Future<void> press(String tooltip) async {
    IconButton? button;
    void find(Element element) {
      final widget = element.widget;
      if (widget is IconButton && widget.tooltip == tooltip) button = widget;
      element.visitChildren(find);
    }

    WidgetsBinding.instance.rootElement!.visitChildren(find);
    check(button?.onPressed != null, 'Missing enabled button: $tooltip');
    button!.onPressed!();
    await Future<void>.delayed(const Duration(milliseconds: 700));
  }

  DanmakuOverlay? danmakuOverlay() {
    DanmakuOverlay? found;
    void find(Element element) {
      if (element.widget is DanmakuOverlay) {
        found = element.widget as DanmakuOverlay;
      }
      element.visitChildren(find);
    }

    WidgetsBinding.instance.rootElement!.visitChildren(find);
    return found;
  }

  int renderedDanmakuCount() {
    var count = 0;
    void find(Element element) {
      if (element.widget is Text &&
          ((element.widget as Text).data ?? '').startsWith('小窗弹幕测试')) {
        count++;
      }
      element.visitChildren(find);
    }

    WidgetsBinding.instance.rootElement!.visitChildren(find);
    return count;
  }

  Future<void> snapshot(String phase) async {
    final bounds = await windowManager.getBounds();
    steps[phase] = {
      'bounds': [bounds.left, bounds.top, bounds.width, bounds.height],
      'topmost': await windowManager.isAlwaysOnTop(),
      'positionMs': player?.state.position.inMilliseconds,
      'playing': player?.state.playing,
      'videoWidth': player?.state.width,
      'videoHeight': player?.state.height,
      'danmakuVisible': danmakuOverlay()?.controller.visible,
      'danmakuMoving': danmakuOverlay()?.controller.moving,
      'renderedDanmaku': renderedDanmakuCount(),
    };
    await File('${report.path}.phase').writeAsString(phase, flush: true);
  }

  Element? keyedElement(String key) {
    Element? found;
    void find(Element element) {
      if (element.widget.key == ValueKey(key)) found = element;
      element.visitChildren(find);
    }

    WidgetsBinding.instance.rootElement!.visitChildren(find);
    return found;
  }

  bool controlsVisible() {
    bool visible = true;
    keyedElement('pip-header')!.visitAncestorElements((element) {
      if (element.widget is IgnorePointer) {
        visible = !(element.widget as IgnorePointer).ignoring;
        return false;
      }
      return true;
    });
    return visible;
  }

  try {
    SharedPreferences.setMockInitialValues({});
    store = LocalStore(await SharedPreferences.getInstance());
    final repository = LocalVideoRepository(arguments[1]);
    final detail = await repository.detail(FixtureRepository.free);
    player = Player(configuration: const PlayerConfiguration(muted: true));
    final activePlayer = player;
    final navigator = GlobalKey<NavigatorState>();
    runApp(
      MaterialApp(
        navigatorKey: navigator,
        home: const Scaffold(body: Center(child: Text('小窗验证完成后返回此页'))),
      ),
    );
    await Future<void>.delayed(const Duration(milliseconds: 700));
    await windowManager.setBounds(const Rect.fromLTWH(100, 80, 960, 700));
    await windowManager.show();
    await windowManager.focus();
    final original = await windowManager.getBounds();
    unawaited(
      navigator.currentState!.push(
        MaterialPageRoute<void>(
          builder: (_) => PlayerScreen(
            detail: detail,
            initialIndex: 0,
            repository: repository,
            store: store!,
            playerFactory: () => activePlayer,
          ),
        ),
      ),
    );
    await waitFor(() => activePlayer.state.position.inMilliseconds >= 1000);
    await snapshot('normal');
    final originalDanmaku = danmakuOverlay()!.controller;
    check(renderedDanmakuCount() > 0, 'Main window danmaku missing');
    await press('小窗播放 (Ctrl+Shift+P)');
    check(await windowManager.isAlwaysOnTop(), 'PiP must be topmost');
    final pip = await windowManager.getBounds();
    check(
      pip.width < original.width && pip.height < original.height,
      'PiP must be smaller than original',
    );
    check(pip.height > pip.width, 'Portrait video must use portrait PiP');
    await snapshot('pip');
    check(
      identical(danmakuOverlay()?.controller, originalDanmaku),
      'PiP replaced danmaku controller',
    );
    check(renderedDanmakuCount() > 0, 'PiP danmaku missing');
    await Future<void>.delayed(const Duration(seconds: 4));
    await press('关闭弹幕');
    check(renderedDanmakuCount() == 0, 'Danmaku did not turn off');
    check(
      !store.playbackPreferences.danmaku,
      'Danmaku preference was not saved',
    );
    await snapshot('danmaku_off');
    await press('开启弹幕');
    check(renderedDanmakuCount() > 0, 'Danmaku did not turn on');
    await snapshot('danmaku_on');
    await press('取消置顶');
    check(!await windowManager.isAlwaysOnTop(), 'Unpin did not clear topmost');
    await snapshot('unpinned');
    final beforeBlur = activePlayer.state.position;
    await windowManager.blur();
    await Future<void>.delayed(const Duration(seconds: 2));
    check(activePlayer.state.playing, 'PiP stopped when losing focus');
    check(
      activePlayer.state.position > beforeBlur,
      'PiP video did not advance',
    );
    await snapshot('blurred');
    await windowManager.focus();
    await windowManager.setSize(const Size(240, 180));
    await Future<void>.delayed(const Duration(milliseconds: 700));
    await snapshot('minimum_size');
    await windowManager.setBounds(pip);
    await press('无边框（纯画面）');
    check(
      keyedElement('pip-borderless') != null,
      'Borderless mode not entered',
    );
    final area =
        keyedElement('pip-video-area')!.findRenderObject() as RenderBox;
    final smallWindow =
        keyedElement('desktop-picture-in-picture')!.findRenderObject()
            as RenderBox;
    check(
      area.size == smallWindow.size,
      'Borderless video retained padding or bars',
    );
    await snapshot('borderless_controls');
    await Future<void>.delayed(const Duration(seconds: 3));
    check(!controlsVisible(), 'Borderless controls did not hide');
    check(renderedDanmakuCount() > 0, 'Hidden controls also hid danmaku');
    check(originalDanmaku.moving, 'Borderless danmaku stopped moving');
    await snapshot('borderless');
    await Future<void>.delayed(const Duration(seconds: 2));
    check(controlsVisible(), 'Mouse movement did not reveal controls');
    await snapshot('borderless_hover');
    await press('置顶小窗');
    check(await windowManager.isAlwaysOnTop(), 'Pin did not enable topmost');
    await snapshot('repinned');
    await press('显示边框与控制栏');
    check(keyedElement('pip-borderless') == null, 'Borderless mode not exited');
    await press('暂停播放');
    check(!activePlayer.state.playing, 'Pause did not work');
    check(!originalDanmaku.moving, 'Paused danmaku kept moving');
    final pausedAt = activePlayer.state.position;
    await press('返回大窗 (Esc)');
    check(!await windowManager.isAlwaysOnTop(), 'Normal window stayed topmost');
    check(
      await windowManager.getBounds() == original,
      'Bounds were not restored',
    );
    check(!activePlayer.state.playing, 'Restoring resumed paused playback');
    check(
      activePlayer.state.position == pausedAt,
      'Restoring changed position',
    );
    await snapshot('restored');
    check(
      identical(danmakuOverlay()?.controller, originalDanmaku),
      'Restoring replaced danmaku controller',
    );
    check(renderedDanmakuCount() > 0, 'Restored danmaku missing');
    await press('小窗播放 (Ctrl+Shift+P)');
    await press('继续播放');
    await windowManager.minimize();
    await Future<void>.delayed(const Duration(seconds: 1));
    check(!activePlayer.state.playing, 'Minimized playback did not pause');
    await windowManager.restore();
    await windowManager.focus();
    await press('返回大窗 (Esc)');
    await press('旋转与全屏');
    check(await windowManager.isFullScreen(), 'Fullscreen entry failed');
    final fullscreenPip = Completer<void>();
    void enterFullscreenPip(Element element) {
      if (element.widget is CallbackShortcuts && !fullscreenPip.isCompleted) {
        final shortcuts = element.widget as CallbackShortcuts;
        for (final entry in shortcuts.bindings.entries) {
          if (entry.key is SingleActivator &&
              (entry.key as SingleActivator).control &&
              (entry.key as SingleActivator).shift) {
            entry.value();
            fullscreenPip.complete();
            return;
          }
        }
      }
      element.visitChildren(enterFullscreenPip);
    }

    WidgetsBinding.instance.rootElement!.visitChildren(enterFullscreenPip);
    await fullscreenPip.future.timeout(const Duration(seconds: 2));
    await Future<void>.delayed(const Duration(seconds: 1));
    check(!await windowManager.isFullScreen(), 'PiP remained fullscreen');
    check(await windowManager.isAlwaysOnTop(), 'Fullscreen PiP is not topmost');
    await press('返回大窗 (Esc)');
    check(await windowManager.isFullScreen(), 'Fullscreen was not restored');
    await snapshot('fullscreen_restored');
    void escapeFullscreen(Element element) {
      final widget = element.widget;
      if (widget is CallbackShortcuts) {
        widget.bindings[const SingleActivator(LogicalKeyboardKey.escape)]
            ?.call();
      }
      element.visitChildren(escapeFullscreen);
    }

    WidgetsBinding.instance.rootElement!.visitChildren(escapeFullscreen);
    await Future<void>.delayed(const Duration(milliseconds: 700));
    await windowManager.maximize();
    await press('小窗播放 (Ctrl+Shift+P)');
    await press('返回大窗 (Esc)');
    check(
      await windowManager.isMaximized(),
      'Maximized state was not restored',
    );
    await snapshot('maximized_restored');
    await press('小窗播放 (Ctrl+Shift+P)');
    await press('关闭小窗并停止播放');
    check(!navigator.currentState!.canPop(), 'Close did not return to library');
    check(!await windowManager.isAlwaysOnTop(), 'Close left window topmost');
    check(errors.isEmpty, errors.join('\n'));
    store.dispose();
    await report.writeAsString(
      jsonEncode({'ok': true, 'steps': steps}),
      flush: true,
    );
    exit(0);
  } catch (error, stack) {
    await report.writeAsString(
      jsonEncode({
        'ok': false,
        'error': '$error',
        'stack': '$stack',
        'flutterErrors': errors,
        'steps': steps,
      }),
      flush: true,
    );
    exit(1);
  }
}
