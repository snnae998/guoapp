// 用模拟的平台通道检查客户端：参数编码、结果解码、错误映射、翻页和日志。
import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:openhongguo/openhongguo.dart';

const channel = MethodChannel('dev.openhongguo/flutter');

/// fake 把平台通道换成 reply：call 的参数解码后交给它，返回值编码成 core 的 JSON。
List<MethodCall> fake(Object? Function(String method, List<dynamic> args) reply, {String create = '{"id":7}'}) {
  final calls = <MethodCall>[];
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger.setMockMethodCallHandler(channel, (call) async {
    calls.add(call);
    final a = call.arguments as Map;
    switch (call.method) {
      case 'createClient':
        return create;
      case 'call':
        final out = reply(a['method'] as String, jsonDecode(a['args'] as String) as List<dynamic>);
        return jsonEncode(out is OpenhongguoException
            ? {'error': {'code': out.code.wire, 'message': out.message, 'retryable': out.retryable, 'retryAfterMs': out.retryAfterMs}}
            : {'result': out});
      default:
        return null;
    }
  });
  return calls;
}

Map<String, dynamic> series(String id) => {'id': id, 'title': '剧 $id', 'episode_count': 80};

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('参数和设置传给原生端，结果解码成类型', () async {
    final calls = fake((_, __) => {'items': [series('1')], 'cursor': 'c2', 'has_more': true});
    final c = await OpenhongguoClient.create(const ClientOptions(apiOrigin: 'http://x', device: Device(model: 'Pixel')));
    final create = calls.first.arguments as Map;
    final config = jsonDecode(create['config'] as String) as Map;
    expect(create['profilePath'], isNull);
    expect(config['apiOrigin'], 'http://x');
    expect((config['device'] as Map)['model'], 'Pixel');

    final page = await c.search('短剧', const SearchOptions(pageSize: 20, filters: [SearchFilter.shortPlay]));
    final call = calls.last.arguments as Map;
    expect(call['client'], 7);
    expect(call['method'], 'search');
    expect(jsonDecode(call['args'] as String), [
      '短剧',
      {'pageSize': 20, 'filters': ['content_type_short_play']},
    ]);
    expect(page.items.single.title, '剧 1');
    expect(page.items.single.episodeCount, 80);
    expect(page.cursor, 'c2');
    expect(page.hasMore, isTrue);
  });

  test('缺省和 null 的字段取零值', () async {
    fake((_, __) => {'id': '1', 'subtitles': null, 'novel': null, 'content_type': 7});
    final s = await (await OpenhongguoClient.create()).getSeries('1');
    expect(s.title, '');
    expect(s.subtitles, isEmpty);
    expect(s.novel, isNull);
    expect(s.contentType, ContentType.shortPlay);
    expect(s.status, SeriesStatus.updating);
  });

  test('错误带着错误码抛出', () async {
    fake((_, __) => const OpenhongguoException(ErrorCode.rateLimited, 'slow', retryable: true, retryAfterMs: 1500));
    final c = await OpenhongguoClient.create();
    await expectLater(
      c.hotSearch(),
      throwsA(isA<OpenhongguoException>()
          .having((e) => e.code, 'code', ErrorCode.rateLimited)
          .having((e) => e.retryable, 'retryable', isTrue)
          .having((e) => e.retryAfterMs, 'retryAfterMs', 1500)),
    );
  });

  test('创建失败时抛出原生端给出的错误', () async {
    fake((_, __) => null, create: '{"error":{"code":"INVALID_OPTIONS","message":"bad device"}}');
    await expectLater(
      OpenhongguoClient.create(),
      throwsA(isA<OpenhongguoException>().having((e) => e.code, 'code', ErrorCode.invalidOptions)),
    );
  });

  test('searchAll 沿游标翻页并去重', () async {
    final calls = fake((_, args) => (args[1] as Map)['cursor'] == 'p2'
        ? {'items': [series('2'), series('3')], 'has_more': false}
        : {'items': [series('1'), series('2')], 'cursor': 'p2', 'has_more': true});
    final c = await OpenhongguoClient.create();
    final ids = await c.searchAll('短剧').map((s) => s.id).toList();
    expect(ids, ['1', '2', '3']);
    expect(calls.where((c) => c.method == 'call'), hasLength(2));
  });

  test('加密剧集推导密钥失败时，播放模型照常返回', () async {
    fake((_, __) => {
          'playback': {'protected': true, 'streams': [{'definition': '720p'}]},
          'contentKeyError': {'code': 'DRM_PROTECTED', 'message': 'bad key'},
        });
    final r = await (await OpenhongguoClient.create()).resolvePlayback('9');
    expect(r.playback.protected, isTrue);
    expect(r.playback.streams.single.definition, '720p');
    expect(r.contentKey, isNull);
    expect(r.contentKeyError?.code, ErrorCode.drmProtected);
  });

  test('日志交给对应的客户端', () async {
    fake((_, __) => null);
    final lines = <String>[];
    await OpenhongguoClient.create(ClientOptions(log: lines.add));
    await TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger.handlePlatformMessage(
      channel.name,
      channel.codec.encodeMethodCall(const MethodCall('log', {'client': 7, 'line': 'HTTP GET'})),
      (_) {},
    );
    expect(lines, ['HTTP GET']);
  });
}
