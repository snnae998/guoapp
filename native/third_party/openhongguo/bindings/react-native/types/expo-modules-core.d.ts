// 编译时用的最小声明，只覆盖本包用到的部分；运行时由 App 提供 expo-modules-core。
declare module 'expo-modules-core' {
  export function requireNativeModule<T>(name: string): T;
}
