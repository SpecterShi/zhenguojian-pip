import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:media_kit/media_kit.dart';

import 'models.dart';
import 'episode_browser.dart';
import 'playback_preferences.dart';
import 'video_enhancement.dart';
import 'video_enhancement_settings.dart';
import 'diary_service.dart';

enum PlayerMenuSection { episodes, speed, quality, subtitles, settings }

class PlayerMenu extends StatefulWidget {
  const PlayerMenu({
    super.key,
    required this.section,
    required this.episodes,
    required this.currentIndex,
    required this.preferences,
    required this.qualities,
    required this.actualQuality,
    required this.local,
    required this.favorite,
    required this.mobile,
    required this.onEpisode,
    required this.onPreferences,
    required this.onFavorite,
    this.showDanmaku = false,
    this.danmakuStatus = '',
    this.onRetryDanmaku,
    this.preloadStatus = '',
    this.enhancement,
    this.onCompareEnhancement,
    this.subtitleTracks = const [],
    this.subtitleTrackId = 'auto',
    this.subtitleDelay = 0,
    this.subtitleFontSize = 55,
    this.onSubtitle,
    this.onSubtitleFile,
    this.onSubtitleDelay,
    this.onSubtitleFontSize,
  });

  final PlayerMenuSection section;
  final List<Episode> episodes;
  final int currentIndex;
  final PlaybackPreferences preferences;
  final List<int> qualities;
  final int actualQuality;
  final bool local;
  final bool favorite;
  final bool mobile;
  final bool showDanmaku;
  final String danmakuStatus;
  final VoidCallback? onRetryDanmaku;
  final String preloadStatus;
  final VideoEnhancementController? enhancement;
  final VoidCallback? onCompareEnhancement;
  final List<SubtitleTrack> subtitleTracks;
  final String subtitleTrackId;
  final double subtitleDelay;
  final int subtitleFontSize;
  final ValueChanged<String>? onSubtitle;
  final Future<void> Function()? onSubtitleFile;
  final Future<void> Function(double delta)? onSubtitleDelay;
  final Future<void> Function(int delta)? onSubtitleFontSize;
  final ValueChanged<int> onEpisode;
  final Future<void> Function(PlaybackPreferences) onPreferences;
  final Future<void> Function() onFavorite;

  @override
  State<PlayerMenu> createState() => _PlayerMenuState();
}

class _PlayerMenuState extends State<PlayerMenu> {
  bool _busy = false;
  String? _error;

  Future<void> _run(Future<void> Function() action) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } catch (_) {
      if (mounted) setState(() => _error = '未能保存设置，请检查存储空间后重试。');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    final landscape = size.width > size.height;
    final colors = Theme.of(context).colorScheme;
    final title = switch (widget.section) {
      PlayerMenuSection.episodes => '选集 · 共 ${widget.episodes.length} 集',
      PlayerMenuSection.speed => '播放倍速',
      PlayerMenuSection.quality => '清晰度',
      PlayerMenuSection.subtitles => '字幕',
      PlayerMenuSection.settings => '播放设置',
    };
    return Dialog(
      key: const ValueKey('player-menu'),
      alignment: landscape ? Alignment.centerRight : Alignment.bottomCenter,
      insetPadding: const EdgeInsets.all(12),
      backgroundColor: colors.surface,
      child: SizedBox(
        width: landscape ? math.min(440, size.width * .6) : 600,
        height: landscape ? size.height : size.height * .72,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(18, 8, 4, 4),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      title,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                  IconButton(
                    tooltip: '关闭菜单',
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
            ),
            if (_busy) const LinearProgressIndicator(minHeight: 2),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 18,
                  vertical: 8,
                ),
                child: Text(
                  _error!,
                  style: const TextStyle(color: Color(0xFFFFB4A8)),
                ),
              ),
            Expanded(
              child: widget.section == PlayerMenuSection.episodes
                  ? PlayerEpisodeGrid(
                      episodes: widget.episodes,
                      currentIndex: widget.currentIndex,
                      keyPrefix: 'menu-episode',
                      onSelected: widget.onEpisode,
                    )
                  : SingleChildScrollView(
                      padding: const EdgeInsets.fromLTRB(18, 4, 18, 20),
                      child: _settings(),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _settings() {
    final preferences = widget.preferences;
    final all = widget.section == PlayerMenuSection.settings;
    final colors = Theme.of(context).colorScheme;
    final helperStyle = TextStyle(
      fontSize: 13,
      color: colors.onSurfaceVariant,
      height: 1.5,
    );
    final qualities = {
      0,
      ...widget.qualities.where((quality) => quality > 0),
    }.toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (all || widget.section == PlayerMenuSection.speed) ...[
          const Text('倍速'),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final speed in playbackSpeeds)
                ChoiceChip(
                  key: ValueKey('menu-speed-$speed'),
                  label: Text('${speed}x'),
                  selected: speed == preferences.speed,
                  onSelected: _busy
                      ? null
                      : (_) => _run(
                          () => widget.onPreferences(
                            preferences.copyWith(speed: speed),
                          ),
                        ),
                ),
            ],
          ),
          const SizedBox(height: 20),
        ],
        if (all || widget.section == PlayerMenuSection.quality) ...[
          const Text('清晰度'),
          const SizedBox(height: 8),
          Text(
            widget.local
                ? '本集正在播放本地原画，画质偏好用于后续在线播放。'
                : widget.actualQuality > 0
                ? '当前播放 ${widget.actualQuality}P'
                : '使用源站可用画质',
            style: helperStyle,
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final quality in qualities)
                ChoiceChip(
                  key: ValueKey('menu-quality-$quality'),
                  label: Text(quality == 0 ? '自动（优先高清）' : '${quality}P'),
                  selected: quality == preferences.quality,
                  onSelected: _busy
                      ? null
                      : (_) => _run(
                          () => widget.onPreferences(
                            preferences.copyWith(quality: quality),
                          ),
                        ),
                ),
            ],
          ),
          if (!qualities.contains(preferences.quality))
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(
                '已保存 ${preferences.quality}P 偏好，本集暂无此画质。',
                style: helperStyle,
              ),
            ),
          const SizedBox(height: 20),
        ],
        if ((all || widget.section == PlayerMenuSection.quality) &&
            widget.enhancement != null)
          VideoEnhancementSettings(
            controller: widget.enhancement!,
            busy: _busy,
            onChanged: (enhancement) => _run(
              () => widget.onPreferences(
                preferences.copyWith(enhancement: enhancement),
              ),
            ),
            onCompare: widget.onCompareEnhancement ?? () {},
          ),
        if (widget.section == PlayerMenuSection.subtitles) ...[
          Text(
            widget.subtitleTracks.isEmpty
                ? '当前视频没有内嵌字幕轨。可以加载外挂字幕文件（.srt / .ass / .ssa / .vtt）。'
                : '当前视频有 ${widget.subtitleTracks.length} 条内嵌字幕轨，也可以加载外挂字幕文件（.srt / .ass / .ssa / .vtt）。',
            style: helperStyle,
          ),
          const SizedBox(height: 12),
          RadioGroup<String>(
            groupValue: widget.subtitleTrackId,
            onChanged: _busy
                ? (_) {}
                : (value) {
                    if (value == null) return;
                    _run(() async => widget.onSubtitle?.call(value));
                  },
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const RadioListTile<String>(
                  key: ValueKey('menu-subtitle-auto'),
                  contentPadding: EdgeInsets.zero,
                  dense: true,
                  title: Text('自动选择'),
                  value: 'auto',
                ),
                const RadioListTile<String>(
                  key: ValueKey('menu-subtitle-no'),
                  contentPadding: EdgeInsets.zero,
                  dense: true,
                  title: Text('关闭字幕'),
                  value: 'no',
                ),
                for (final track in widget.subtitleTracks.where(
                  (track) => !track.uri && !track.data,
                ))
                  RadioListTile<String>(
                    key: ValueKey('menu-subtitle-${track.id}'),
                    contentPadding: EdgeInsets.zero,
                    dense: true,
                    title: Text(
                      track.title?.isNotEmpty == true
                          ? track.title!
                          : track.language?.isNotEmpty == true
                          ? track.language!
                          : '字幕轨 ${track.id}',
                    ),
                    subtitle: track.title?.isNotEmpty == true &&
                            track.language?.isNotEmpty == true
                        ? Text(track.language!)
                        : null,
                    value: track.id,
                  ),
              ],
            ),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            key: const ValueKey('menu-subtitle-file'),
            onPressed: _busy
                ? null
                : () => _run(widget.onSubtitleFile ?? () async {}),
            icon: const Icon(Icons.folder_open_rounded),
            label: const Text('打开字幕文件…'),
          ),
          const SizedBox(height: 20),
          Text(
            '字幕延迟：${widget.subtitleDelay > 0 ? '+' : ''}${widget.subtitleDelay.toStringAsFixed(1)} 秒'
            '（字幕比画面快就选"延后"，慢就选"提前"）',
            style: helperStyle,
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton(
                key: const ValueKey('menu-subtitle-earlier'),
                onPressed: () => widget.onSubtitleDelay?.call(-0.5),
                child: const Text('提前 0.5s'),
              ),
              OutlinedButton(
                key: const ValueKey('menu-subtitle-later'),
                onPressed: () => widget.onSubtitleDelay?.call(0.5),
                child: const Text('延后 0.5s'),
              ),
              OutlinedButton(
                key: const ValueKey('menu-subtitle-delay-reset'),
                onPressed: widget.subtitleDelay == 0
                    ? null
                    : () => widget.onSubtitleDelay?.call(-widget.subtitleDelay),
                child: const Text('重置延迟'),
              ),
            ],
          ),
          const SizedBox(height: 20),
          Text('字幕大小：${widget.subtitleFontSize}', style: helperStyle),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              OutlinedButton(
                key: const ValueKey('menu-subtitle-smaller'),
                onPressed: widget.subtitleFontSize <= 30
                    ? null
                    : () => widget.onSubtitleFontSize?.call(-5),
                child: const Text('减小'),
              ),
              OutlinedButton(
                key: const ValueKey('menu-subtitle-larger'),
                onPressed: widget.subtitleFontSize >= 90
                    ? null
                    : () => widget.onSubtitleFontSize?.call(5),
                child: const Text('增大'),
              ),
            ],
          ),
        ],
        if (all && widget.showDanmaku) ...[
          SwitchListTile.adaptive(
            key: const ValueKey('player-danmaku-enabled'),
            contentPadding: EdgeInsets.zero,
            title: const Text('弹幕'),
            subtitle: Text(widget.danmakuStatus),
            value: preferences.danmaku,
            onChanged: _busy
                ? null
                : (value) => _run(
                    () => widget.onPreferences(
                      preferences.copyWith(danmaku: value),
                    ),
                  ),
          ),
          if (widget.onRetryDanmaku != null)
            TextButton.icon(
              key: const ValueKey('player-danmaku-retry'),
              onPressed: _busy ? null : widget.onRetryDanmaku,
              icon: const Icon(Icons.refresh_rounded),
              label: const Text('重试弹幕'),
            ),
          const SizedBox(height: 8),
        ],
        if (all) ...[
          SwitchListTile.adaptive(
            key: const ValueKey('player-preload-enabled'),
            contentPadding: EdgeInsets.zero,
            title: const Text('下一集预加载'),
            subtitle: Text(widget.local ? '本地播放不预取网络视频' : widget.preloadStatus),
            value: preferences.preload,
            onChanged: _busy
                ? null
                : (value) => _run(
                    () => widget.onPreferences(
                      preferences.copyWith(preload: value),
                    ),
                  ),
          ),
          const SizedBox(height: 8),
          SwitchListTile.adaptive(
            key: const ValueKey('player-auto-advance'),
            contentPadding: EdgeInsets.zero,
            title: const Text('自动连播'),
            subtitle: const Text('关闭后，播放结束停在当前集'),
            value: preferences.autoAdvance,
            onChanged: _busy
                ? null
                : (value) => _run(
                    () => widget.onPreferences(
                      preferences.copyWith(autoAdvance: value),
                    ),
                  ),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            key: const ValueKey('player-favorite'),
            onPressed: _busy ? null : () => _run(widget.onFavorite),
            icon: Icon(
              widget.favorite
                  ? Icons.bookmark_rounded
                  : Icons.bookmark_border_rounded,
            ),
            label: Text(widget.favorite ? '取消追剧' : '加入追剧'),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            key: const ValueKey('player-diary-btn'),
            onPressed: () => DiaryService.showDiaryDialog(context),
            icon: const Icon(Icons.receipt_long_rounded),
            label: const Text('查看播放调试日记'),
          ),
          const SizedBox(height: 20),
          Text(
            widget.mobile
                ? '上下滑切集；长按画面临时 3 倍速，松开恢复。竖屏轻点暂停，横屏轻点显示控制；双击播放或暂停。'
                : '空格：播放 / 暂停\n左右键：后退 / 快进 5 秒\n长按右键或画面：临时 3 倍速\n上下键：音量 ±5%，M：静音\nF、F11、Ctrl+F：全屏\nEsc：先关闭菜单，再退出全屏',
            style: helperStyle.copyWith(height: 1.6),
          ),
        ],
      ],
    );
  }
}

class PlayerEpisodeGrid extends StatelessWidget {
  const PlayerEpisodeGrid({
    super.key,
    required this.episodes,
    required this.currentIndex,
    required this.onSelected,
    this.keyPrefix = 'play-episode',
    this.compact = false,
    this.title = '选集',
  });
  final List<Episode> episodes;
  final int currentIndex;
  final ValueChanged<int> onSelected;
  final String keyPrefix;
  final bool compact;
  final String title;
  @override
  Widget build(BuildContext context) => EpisodeBrowser(
    episodes: episodes,
    currentNumber: episodes.isEmpty
        ? null
        : episodes[currentIndex.clamp(0, episodes.length - 1)].number,
    onSelected: onSelected,
    keyPrefix: keyPrefix,
    compact: compact,
    title: title,
  );
}
