import 'dart:ui' show PointerDeviceKind;

import 'package:duanju_app/desktop_picture_in_picture.dart';
import 'package:duanju_app/danmaku_overlay.dart';
import 'package:duanju_app/local_store.dart';
import 'package:duanju_app/player_screen.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'fixtures.dart';
import 'danmaku_fixtures.dart';
import 'player_fixtures.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const channel = MethodChannel('window_manager');
  late List<MethodCall> calls;
  late bool fullscreen;
  late bool maximized;
  late bool alwaysOnTop;
  late bool failBounds;

  setUp(() {
    calls = [];
    fullscreen = false;
    maximized = false;
    alwaysOnTop = false;
    failBounds = false;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call);
          switch (call.method) {
            case 'isFullScreen':
              return fullscreen;
            case 'isMaximized':
              return maximized;
            case 'isAlwaysOnTop':
              return alwaysOnTop;
            case 'setFullScreen':
              fullscreen = call.arguments['isFullScreen'];
            case 'maximize':
              maximized = true;
            case 'unmaximize':
              maximized = false;
            case 'setAlwaysOnTop':
              alwaysOnTop = call.arguments['isAlwaysOnTop'];
            case 'getBounds':
              return {'x': 100.0, 'y': 80.0, 'width': 960.0, 'height': 700.0};
            case 'setBounds':
              if (failBounds) {
                failBounds = false;
                throw PlatformException(code: 'synthetic_failure');
              }
          }
          return null;
        });
  });

  tearDown(() {
    debugDefaultTargetPlatformOverride = null;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  testWidgets('small window preserves playback and survives losing focus', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    SharedPreferences.setMockInitialValues({});
    final store = LocalStore(await SharedPreferences.getInstance());
    final repository = RouteRepository();
    final player = ScriptedPlayer();
    final detail = await repository.detail(FixtureRepository.free);
    Future<void> settle() async {
      for (var i = 0; i < 30; i++) {
        await tester.pump(const Duration(milliseconds: 20));
      }
    }

    await tester.pumpWidget(
      MaterialApp(
        home: PlayerScreen(
          detail: detail,
          initialIndex: 0,
          repository: repository,
          store: store,
          playerFactory: () => Player(platformPlayer: player),
          videoBuilder: (controls) => controls,
        ),
      ),
    );
    await settle();
    await player.seek(const Duration(seconds: 37));
    await tester.tap(find.byTooltip('小窗播放 (Ctrl+Shift+P)'));
    await settle();
    expect(
      find.byKey(const ValueKey('desktop-picture-in-picture')),
      findsOneWidget,
    );
    expect(alwaysOnTop, isTrue);
    expect(player.opened, hasLength(1));
    expect(player.state.position, const Duration(seconds: 37));
    await tester.tap(find.byTooltip('取消置顶'));
    await settle();
    expect(alwaysOnTop, isFalse);
    expect(find.byTooltip('置顶小窗'), findsOneWidget);
    await tester.tap(find.byTooltip('无边框（纯画面）'));
    await settle();
    expect(find.byKey(const ValueKey('pip-borderless')), findsOneWidget);
    expect(
      tester.getSize(find.byKey(const ValueKey('pip-video-area'))),
      tester.getSize(find.byKey(const ValueKey('desktop-picture-in-picture'))),
    );
    await tester.pump(const Duration(seconds: 3));
    await settle();
    expect(find.byTooltip('显示边框与控制栏').hitTestable(), findsNothing);
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await mouse.addPointer(location: const Offset(400, 300));
    await mouse.moveTo(const Offset(410, 310));
    await settle();
    expect(find.byTooltip('显示边框与控制栏').hitTestable(), findsOneWidget);
    await tester.tap(find.byTooltip('置顶小窗'));
    await settle();
    expect(alwaysOnTop, isTrue);
    await tester.tap(find.byTooltip('取消置顶'));
    await settle();
    expect(alwaysOnTop, isFalse);
    await tester.tap(find.byTooltip('显示边框与控制栏'));
    await settle();
    expect(find.byKey(const ValueKey('pip-borderless')), findsNothing);
    await mouse.removePointer();
    expect(player.opened, hasLength(1));
    expect(player.state.position, const Duration(seconds: 37));
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await settle();
    expect(player.state.playing, isTrue);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.tap(find.byTooltip('暂停播放'));
    await settle();
    expect(player.state.playing, isFalse);
    await tester.tap(find.byTooltip('返回大窗 (Esc)'));
    await settle();
    expect(
      find.byKey(const ValueKey('desktop-picture-in-picture')),
      findsNothing,
    );
    expect(alwaysOnTop, isFalse);
    expect(player.state.playing, isFalse);
    expect(player.opened, hasLength(1));
    expect(player.state.position, const Duration(seconds: 37));
    await tester.tap(find.byTooltip('小窗播放 (Ctrl+Shift+P)'));
    await settle();
    expect(alwaysOnTop, isFalse);
    expect(find.byTooltip('置顶小窗'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    await settle();
    expect(player.disposed, isTrue);
    expect(tester.takeException(), isNull);
    store.dispose();
    debugDefaultTargetPlatformOverride = null;
  });

  testWidgets(
    'PiP retains danmaku through borderless, toggle, pause and episode changes',
    (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.windows;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      SharedPreferences.setMockInitialValues({});
      final store = LocalStore(await SharedPreferences.getInstance());
      final repository = DanmakuFixtureRepository();
      final player = ScriptedPlayer();
      final detail = await repository.detail(FixtureRepository.free);
      Future<void> settle() async {
        for (var i = 0; i < 20; i++) {
          await tester.pump(const Duration(milliseconds: 20));
        }
      }

      await tester.pumpWidget(
        MaterialApp(
          home: PlayerScreen(
            detail: detail,
            initialIndex: 0,
            repository: repository,
            store: store,
            playerFactory: () => Player(platformPlayer: player),
            videoBuilder: (controls) => controls,
          ),
        ),
      );
      await settle();
      await player.seek(const Duration(seconds: 2));
      await settle();
      expect(find.text('合成弹幕 1001'), findsOneWidget);
      final controller = tester
          .widget<DanmakuOverlay>(find.byType(DanmakuOverlay))
          .controller;
      final requestCount = repository.danmakuCalls.length;
      await tester.tap(find.byTooltip('小窗播放 (Ctrl+Shift+P)'));
      tester.view.physicalSize = const Size(300, 625);
      await settle();
      expect(find.text('合成弹幕 1001'), findsOneWidget);
      expect(
        tester.widget<DanmakuOverlay>(find.byType(DanmakuOverlay)).controller,
        same(controller),
      );
      expect(repository.danmakuCalls.length, requestCount);
      expect(find.byKey(const ValueKey('player-danmaku-toggle')), findsNothing);
      await tester.tap(find.byTooltip('无边框（纯画面）'));
      await settle();
      await tester.pump(const Duration(seconds: 3));
      await settle();
      expect(
        find.byKey(const ValueKey('pip-danmaku-toggle')).hitTestable(),
        findsNothing,
      );
      expect(find.text('合成弹幕 1001'), findsOneWidget);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      await settle();
      expect(controller.visible, isTrue);
      expect(controller.moving, isTrue);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);

      await tester.tapAt(const Offset(150, 300));
      await settle();
      await tester.tap(find.byKey(const ValueKey('pip-danmaku-toggle')));
      await settle();
      expect(find.text('合成弹幕 1001'), findsNothing);
      expect(store.playbackPreferences.danmaku, isFalse);
      expect(find.byTooltip('开启弹幕'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('pip-danmaku-toggle')));
      await settle();
      expect(store.playbackPreferences.danmaku, isTrue);
      expect(find.text('合成弹幕 1001'), findsOneWidget);
      await tester.tap(find.byTooltip('暂停播放'));
      await settle();
      expect(controller.visible, isTrue);
      expect(controller.moving, isFalse);
      final transform = find.byKey(const ValueKey('danmaku-motion-1001-0'));
      final pausedX = tester.widget<Transform>(transform).transform.storage[12];
      await tester.pump(const Duration(seconds: 1));
      expect(
        tester.widget<Transform>(transform).transform.storage[12],
        pausedX,
      );
      await player.seek(const Duration(seconds: 35));
      await settle();
      expect(
        find.byKey(const ValueKey('danmaku-item-1001-30000')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('danmaku-item-1001-0')), findsNothing);
      await tester.tapAt(const Offset(150, 300));
      await settle();
      await tester.tap(find.byTooltip('下一集'));
      await settle();
      expect(find.text('合成弹幕 1001'), findsNothing);
      expect(find.text('合成弹幕 1002'), findsOneWidget);
      await tester.tap(find.byTooltip('返回大窗 (Esc)'));
      tester.view.resetPhysicalSize();
      await settle();
      expect(find.text('合成弹幕 1002'), findsOneWidget);
      expect(
        tester.widget<DanmakuOverlay>(find.byType(DanmakuOverlay)).controller,
        same(controller),
      );
      expect(player.opened, hasLength(2));
      await tester.pumpWidget(const SizedBox.shrink());
      await settle();
      expect(tester.takeException(), isNull);
      expect(player.disposed, isTrue);
      store.dispose();
      debugDefaultTargetPlatformOverride = null;
    },
  );

  test('portrait and landscape windows remain inside original bounds', () {
    const original = Rect.fromLTWH(-1200, 80, 960, 700);
    for (final ratio in [9 / 16, 16 / 9, double.nan, 0.01]) {
      final bounds = desktopPictureInPictureBounds(original, ratio);
      expect(bounds.width, greaterThanOrEqualTo(240));
      expect(bounds.height, greaterThanOrEqualTo(180));
      expect(original.contains(bounds.topLeft), isTrue);
      expect(bounds.right, lessThanOrEqualTo(original.right));
      expect(bounds.bottom, lessThanOrEqualTo(original.bottom));
    }
    expect(
      desktopPictureInPictureBounds(original, 9 / 16).height,
      greaterThan(desktopPictureInPictureBounds(original, 9 / 16).width),
    );
  });

  test(
    'restores bounds, topmost and fullscreen after entering from fullscreen',
    () async {
      fullscreen = true;
      maximized = true;
      final window = DesktopPictureInPictureWindow();
      await window.enter(9 / 16);
      expect(alwaysOnTop, isTrue);
      expect(fullscreen, isFalse);
      expect(maximized, isFalse);
      await window.restore();
      expect(alwaysOnTop, isFalse);
      expect(fullscreen, isTrue);
      expect(maximized, isTrue);
      expect(window.needsRestore, isFalse);
      final restored = calls
          .lastWhere((call) => call.method == 'setBounds')
          .arguments;
      expect(restored['x'], 100);
      expect(restored['y'], 80);
      expect(restored['width'], 960);
      expect(restored['height'], 700);
    },
  );

  test('restores the original window if resizing fails', () async {
    failBounds = true;
    final window = DesktopPictureInPictureWindow();
    await expectLater(window.enter(16 / 9), throwsA(isA<PlatformException>()));
    expect(alwaysOnTop, isFalse);
    expect(window.needsRestore, isFalse);
    expect(calls.any((call) => call.method == 'setTitleBarStyle'), isTrue);
  });

  test('unpinned PiP restores an originally topmost main window', () async {
    alwaysOnTop = true;
    final window = DesktopPictureInPictureWindow();
    await window.enter(16 / 9, alwaysOnTop: false);
    expect(alwaysOnTop, isFalse);
    await window.restore();
    expect(alwaysOnTop, isTrue);
  });
}
