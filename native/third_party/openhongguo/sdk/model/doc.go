// Package model 存放 sdk 包返回的值：短剧、分集、播放模型、榜单，
// 以及客户端出示的匿名设备。
//
// 各枚举（SeriesStatus、PayType、ContentType、PlatformType、InfoKind、
// RankChangeKind）复现官方 App 自带的枚举，取值沿用服务端的数字；
// 它们的 String 方法给出 CLI 打印的名称。
//
// 热度、播放量等计数是服务端的原始数字。服务端只给展示文本时（"378万热度"），
// 字段是字符串。签名媒体 URL 会过期：CoverExpiresAt 和 Playback.ExpiresAt
// 给出过期时间。
package model
