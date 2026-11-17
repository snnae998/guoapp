// react-native-openhongguo：红果短剧（Hongguo）的非官方 React Native SDK。
//
// 原生层是 gomobile 编译的 Go SDK（Android 的 .so，iOS 的 xcframework），
// 通过 Expo Modules 接入。接口与 Node.js 版 openhongguo 一致。

import { createClientWith, type Client, type ClientOptions } from './client';
import native from './native';

export { Client, OpenhongguoError, SearchFilter } from './client';
export type { ClientOptions, ErrorCode, PlaybackResult } from './client';
export type * from './types';

/**
 * createClient 创建一个客户端。首次请求时，SDK 注册一台匿名设备，
 * 并保存在 options.profilePath（默认在 App 不参与备份的目录里）。
 */
export function createClient(options: ClientOptions = {}): Promise<Client> {
  return createClientWith(native, options);
}
