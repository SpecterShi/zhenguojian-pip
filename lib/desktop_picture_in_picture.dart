import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:media_kit/media_kit.dart';
import 'package:window_manager/window_manager.dart';

Rect desktopPictureInPictureBounds(Rect original, double aspectRatio) {
  final ratio = aspectRatio.isFinite && aspectRatio > 0
      ? aspectRatio.clamp(0.4, 2.5).toDouble()
      : 16 / 9;
  var width = ratio < 1 ? 300.0 : 480.0;
  var height = width / ratio + 92;
  final availableWidth = math.max(240.0, original.width - 24);
  final availableHeight = math.max(180.0, original.height - 24);
  final scale = math.min(
    1.0,
    math.min(availableWidth / width, availableHeight / height),
  );
  width = math.max(240.0, width * scale);
  height = math.max(180.0, height * scale);
  return Rect.fromLTWH(
    original.right - width,
    original.bottom - height,
    width,
    height,
  );
}

class DesktopPictureInPictureWindow {
  Rect? _bounds;
  bool _maximized = false;
  bool _fullscreen = false;
  bool _alwaysOnTop = false;
  bool get needsRestore => _bounds != null;

  Future<void> enter(double aspectRatio, {bool alwaysOnTop = true}) async {
    _fullscreen = await windowManager.isFullScreen();
    _maximized = await windowManager.isMaximized();
    _alwaysOnTop = await windowManager.isAlwaysOnTop();
    _bounds = await windowManager.getBounds();
    try {
      if (_fullscreen) await windowManager.setFullScreen(false);
      if (_maximized) await windowManager.unmaximize();
      _bounds = await windowManager.getBounds();
      await windowManager.setAsFrameless();
      await windowManager.setMinimumSize(const Size(240, 180));
      await windowManager.setAlwaysOnTop(alwaysOnTop);
      await windowManager.setBounds(
        desktopPictureInPictureBounds(_bounds!, aspectRatio),
      );
    } catch (_) {
      await restore();
      rethrow;
    }
  }

  Future<void> restore() async {
    final bounds = _bounds;
    if (bounds == null) return;
    await windowManager.setAlwaysOnTop(_alwaysOnTop);
    if (await windowManager.isFullScreen()) {
      await windowManager.setFullScreen(false);
    }
    await windowManager.setTitleBarStyle(TitleBarStyle.normal);
    await windowManager.setMinimumSize(Size.zero);
    if (await windowManager.isMaximized()) await windowManager.unmaximize();
    await windowManager.setBounds(bounds);
    if (_maximized) await windowManager.maximize();
    if (_fullscreen) await windowManager.setFullScreen(true);
    _bounds = null;
  }
}

class DesktopPictureInPicture extends StatefulWidget {
  const DesktopPictureInPicture({
    super.key,
    required this.player,
    required this.title,
    required this.video,
    required this.enabled,
    required this.onRestore,
    required this.onClose,
    required this.onTogglePlayback,
    required this.onSeek,
    required this.alwaysOnTop,
    required this.onToggleAlwaysOnTop,
    this.danmakuEnabled = true,
    this.onToggleDanmaku,
    this.onPrevious,
    this.onNext,
    this.loading = false,
    this.error,
    this.onRetry,
  });

  final Player player;
  final String title;
  final Widget video;
  final bool enabled;
  final bool loading;
  final String? error;
  final VoidCallback onRestore;
  final VoidCallback onClose;
  final VoidCallback onTogglePlayback;
  final VoidCallback? onPrevious;
  final VoidCallback? onNext;
  final VoidCallback? onRetry;
  final Future<void> Function(Duration) onSeek;
  final bool alwaysOnTop;
  final VoidCallback? onToggleAlwaysOnTop;
  final bool danmakuEnabled;
  final VoidCallback? onToggleDanmaku;

  @override
  State<DesktopPictureInPicture> createState() =>
      _DesktopPictureInPictureState();
}

class _DesktopPictureInPictureState extends State<DesktopPictureInPicture> {
  final _subscriptions = <StreamSubscription<dynamic>>[];
  double? _seekValue;
  bool _borderless = false;
  bool _overlayVisible = true;
  Timer? _overlayTimer;

  void _showOverlay() {
    if (!_borderless) return;
    _overlayTimer?.cancel();
    if (!_overlayVisible) setState(() => _overlayVisible = true);
    _overlayTimer = Timer(const Duration(seconds: 2), _hideOverlay);
  }

  void _hideOverlay() {
    _overlayTimer?.cancel();
    if (mounted && _borderless && _seekValue == null) {
      setState(() => _overlayVisible = false);
    }
  }

  void _toggleBorderless() {
    _overlayTimer?.cancel();
    setState(() {
      _borderless = !_borderless;
      _overlayVisible = true;
    });
    _showOverlay();
  }

  @override
  void initState() {
    super.initState();
    for (final stream in [
      widget.player.stream.position,
      widget.player.stream.duration,
      widget.player.stream.playing,
    ]) {
      _subscriptions.add(
        stream.listen((_) {
          if (mounted) setState(() {});
        }),
      );
    }
  }

  @override
  void dispose() {
    _overlayTimer?.cancel();
    for (final subscription in _subscriptions) {
      subscription.cancel();
    }
    super.dispose();
  }

  String _time(Duration value) {
    final minutes = value.inMinutes.toString().padLeft(2, '0');
    final seconds = (value.inSeconds % 60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  Widget _button(
    String tooltip,
    IconData icon,
    VoidCallback? onPressed, {
    bool selected = false,
  }) => IconButton(
    tooltip: tooltip,
    onPressed: onPressed,
    icon: Icon(icon, size: 19),
    color: selected ? const Color(0xFF80D8FF) : null,
    padding: EdgeInsets.zero,
    constraints: const BoxConstraints.tightFor(width: 32, height: 32),
    visualDensity: VisualDensity.compact,
  );

  Widget _header() => SizedBox(
    key: const ValueKey('pip-header'),
    height: 32,
    child: Row(
      children: [
        Expanded(
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onPanStart: (_) => windowManager.startDragging(),
            onDoubleTap: widget.onRestore,
            child: Tooltip(
              message: '拖动移动小窗，双击返回大窗',
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 7),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    widget.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12),
                  ),
                ),
              ),
            ),
          ),
        ),
        if (widget.onToggleDanmaku != null)
          IconButton(
            key: const ValueKey('pip-danmaku-toggle'),
            tooltip: widget.danmakuEnabled ? '关闭弹幕' : '开启弹幕',
            onPressed: widget.onToggleDanmaku,
            icon: Text(
              '弹',
              style: TextStyle(
                fontSize: 16,
                color: widget.danmakuEnabled
                    ? const Color(0xFF80D8FF)
                    : Colors.white70,
              ),
            ),
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints.tightFor(width: 32, height: 32),
            visualDensity: VisualDensity.compact,
          ),
        _button(
          widget.alwaysOnTop ? '取消置顶' : '置顶小窗',
          widget.alwaysOnTop ? Icons.push_pin : Icons.push_pin_outlined,
          widget.onToggleAlwaysOnTop,
          selected: widget.alwaysOnTop,
        ),
        _button(
          _borderless ? '显示边框与控制栏' : '无边框（纯画面）',
          _borderless ? Icons.web_asset_rounded : Icons.crop_free_rounded,
          _toggleBorderless,
          selected: _borderless,
        ),
        _button('返回大窗 (Esc)', Icons.open_in_full_rounded, widget.onRestore),
        _button('关闭小窗并停止播放', Icons.close_rounded, widget.onClose),
      ],
    ),
  );

  Widget _video() => Stack(
    key: const ValueKey('pip-video-area'),
    fit: StackFit.expand,
    children: [
      widget.video,
      if (widget.loading) const Center(child: CircularProgressIndicator()),
      if (widget.error != null)
        ColoredBox(
          color: const Color(0xE6000000),
          child: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text('暂时无法播放'),
                TextButton(onPressed: widget.onRetry, child: const Text('重试')),
              ],
            ),
          ),
        ),
    ],
  );

  Widget _floatingControls(Widget child) => IgnorePointer(
    ignoring: !_overlayVisible,
    child: AnimatedOpacity(
      opacity: _overlayVisible ? 1 : 0,
      duration: const Duration(milliseconds: 140),
      child: ColoredBox(color: const Color(0xD9171717), child: child),
    ),
  );

  Widget _footer() {
    final state = widget.player.state;
    final duration = state.duration.inMilliseconds.toDouble();
    final position = (_seekValue ?? state.position.inMilliseconds.toDouble())
        .clamp(0.0, math.max(1.0, duration))
        .toDouble();
    return SizedBox(
      key: const ValueKey('pip-footer'),
      height: 46,
      child: Row(
        children: [
          _button(
            '上一集',
            Icons.skip_previous_rounded,
            widget.enabled ? widget.onPrevious : null,
          ),
          _button(
            state.playing ? '暂停播放' : '继续播放',
            state.playing ? Icons.pause_rounded : Icons.play_arrow_rounded,
            widget.enabled ? widget.onTogglePlayback : null,
          ),
          _button(
            '下一集',
            Icons.skip_next_rounded,
            widget.enabled ? widget.onNext : null,
          ),
          Expanded(
            child: SliderTheme(
              data: SliderTheme.of(context).copyWith(
                trackHeight: 2,
                thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 5),
                overlayShape: const RoundSliderOverlayShape(overlayRadius: 10),
              ),
              child: Slider(
                value: position,
                max: math.max(1, duration),
                onChanged: widget.enabled && duration > 0
                    ? (value) {
                        _overlayTimer?.cancel();
                        setState(() => _seekValue = value);
                      }
                    : null,
                onChangeEnd: (value) {
                  setState(() => _seekValue = null);
                  _showOverlay();
                  unawaited(
                    widget.onSeek(Duration(milliseconds: value.round())),
                  );
                },
              ),
            ),
          ),
          Text(_time(state.position), style: const TextStyle(fontSize: 10)),
          const SizedBox(width: 5),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final content = _borderless
        ? Stack(
            key: const ValueKey('pip-borderless'),
            fit: StackFit.expand,
            children: [
              GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: _showOverlay,
                onPanStart: (_) => windowManager.startDragging(),
                child: _video(),
              ),
              Positioned(
                top: 7,
                left: 7,
                right: 7,
                child: _floatingControls(_header()),
              ),
              Positioned(
                bottom: 7,
                left: 7,
                right: 7,
                child: _floatingControls(_footer()),
              ),
            ],
          )
        : Padding(
            padding: const EdgeInsets.all(7),
            child: Column(
              children: [
                _header(),
                Expanded(child: _video()),
                _footer(),
              ],
            ),
          );
    return Theme(
      data: ThemeData.dark(),
      child: Material(
        color: _borderless ? Colors.black : const Color(0xFF171717),
        child: MouseRegion(
          onEnter: (_) => _showOverlay(),
          onHover: (_) => _showOverlay(),
          onExit: (_) => _hideOverlay(),
          child: DragToResizeArea(resizeEdgeSize: 7, child: content),
        ),
      ),
    );
  }
}
