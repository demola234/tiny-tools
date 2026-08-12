// coverage:ignore-file
// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'events.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

T _$identity<T>(T value) => value;

final _privateConstructorUsedError = UnsupportedError(
  'It seems like you constructed your class using `MyClass._()`. This constructor is only meant to be used by freezed and you are not supposed to need it nor use it.\nPlease check the documentation here for more information: https://github.com/rrousselGit/freezed#adding-getters-and-methods-to-our-models',
);

/// @nodoc
mixin _$StorageEvent {
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) => throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) => throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) => throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) => throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) => throw _privateConstructorUsedError;
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) => throw _privateConstructorUsedError;
}

/// @nodoc
abstract class $StorageEventCopyWith<$Res> {
  factory $StorageEventCopyWith(
    StorageEvent value,
    $Res Function(StorageEvent) then,
  ) = _$StorageEventCopyWithImpl<$Res, StorageEvent>;
}

/// @nodoc
class _$StorageEventCopyWithImpl<$Res, $Val extends StorageEvent>
    implements $StorageEventCopyWith<$Res> {
  _$StorageEventCopyWithImpl(this._value, this._then);

  // ignore: unused_field
  final $Val _value;
  // ignore: unused_field
  final $Res Function($Val) _then;

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
}

/// @nodoc
abstract class _$$StorageEvent_KeyCreatedImplCopyWith<$Res> {
  factory _$$StorageEvent_KeyCreatedImplCopyWith(
    _$StorageEvent_KeyCreatedImpl value,
    $Res Function(_$StorageEvent_KeyCreatedImpl) then,
  ) = __$$StorageEvent_KeyCreatedImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String key});
}

/// @nodoc
class __$$StorageEvent_KeyCreatedImplCopyWithImpl<$Res>
    extends _$StorageEventCopyWithImpl<$Res, _$StorageEvent_KeyCreatedImpl>
    implements _$$StorageEvent_KeyCreatedImplCopyWith<$Res> {
  __$$StorageEvent_KeyCreatedImplCopyWithImpl(
    _$StorageEvent_KeyCreatedImpl _value,
    $Res Function(_$StorageEvent_KeyCreatedImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({Object? key = null}) {
    return _then(
      _$StorageEvent_KeyCreatedImpl(
        key:
            null == key
                ? _value.key
                : key // ignore: cast_nullable_to_non_nullable
                    as String,
      ),
    );
  }
}

/// @nodoc

class _$StorageEvent_KeyCreatedImpl extends StorageEvent_KeyCreated {
  const _$StorageEvent_KeyCreatedImpl({required this.key}) : super._();

  @override
  final String key;

  @override
  String toString() {
    return 'StorageEvent.keyCreated(key: $key)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$StorageEvent_KeyCreatedImpl &&
            (identical(other.key, key) || other.key == key));
  }

  @override
  int get hashCode => Object.hash(runtimeType, key);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  @pragma('vm:prefer-inline')
  _$$StorageEvent_KeyCreatedImplCopyWith<_$StorageEvent_KeyCreatedImpl>
  get copyWith => __$$StorageEvent_KeyCreatedImplCopyWithImpl<
    _$StorageEvent_KeyCreatedImpl
  >(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) {
    return keyCreated(key);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) {
    return keyCreated?.call(key);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyCreated != null) {
      return keyCreated(key);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) {
    return keyCreated(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) {
    return keyCreated?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyCreated != null) {
      return keyCreated(this);
    }
    return orElse();
  }
}

abstract class StorageEvent_KeyCreated extends StorageEvent {
  const factory StorageEvent_KeyCreated({required final String key}) =
      _$StorageEvent_KeyCreatedImpl;
  const StorageEvent_KeyCreated._() : super._();

  String get key;

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  _$$StorageEvent_KeyCreatedImplCopyWith<_$StorageEvent_KeyCreatedImpl>
  get copyWith => throw _privateConstructorUsedError;
}

/// @nodoc
abstract class _$$StorageEvent_KeyUpdatedImplCopyWith<$Res> {
  factory _$$StorageEvent_KeyUpdatedImplCopyWith(
    _$StorageEvent_KeyUpdatedImpl value,
    $Res Function(_$StorageEvent_KeyUpdatedImpl) then,
  ) = __$$StorageEvent_KeyUpdatedImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String key});
}

/// @nodoc
class __$$StorageEvent_KeyUpdatedImplCopyWithImpl<$Res>
    extends _$StorageEventCopyWithImpl<$Res, _$StorageEvent_KeyUpdatedImpl>
    implements _$$StorageEvent_KeyUpdatedImplCopyWith<$Res> {
  __$$StorageEvent_KeyUpdatedImplCopyWithImpl(
    _$StorageEvent_KeyUpdatedImpl _value,
    $Res Function(_$StorageEvent_KeyUpdatedImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({Object? key = null}) {
    return _then(
      _$StorageEvent_KeyUpdatedImpl(
        key:
            null == key
                ? _value.key
                : key // ignore: cast_nullable_to_non_nullable
                    as String,
      ),
    );
  }
}

/// @nodoc

class _$StorageEvent_KeyUpdatedImpl extends StorageEvent_KeyUpdated {
  const _$StorageEvent_KeyUpdatedImpl({required this.key}) : super._();

  @override
  final String key;

  @override
  String toString() {
    return 'StorageEvent.keyUpdated(key: $key)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$StorageEvent_KeyUpdatedImpl &&
            (identical(other.key, key) || other.key == key));
  }

  @override
  int get hashCode => Object.hash(runtimeType, key);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  @pragma('vm:prefer-inline')
  _$$StorageEvent_KeyUpdatedImplCopyWith<_$StorageEvent_KeyUpdatedImpl>
  get copyWith => __$$StorageEvent_KeyUpdatedImplCopyWithImpl<
    _$StorageEvent_KeyUpdatedImpl
  >(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) {
    return keyUpdated(key);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) {
    return keyUpdated?.call(key);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyUpdated != null) {
      return keyUpdated(key);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) {
    return keyUpdated(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) {
    return keyUpdated?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyUpdated != null) {
      return keyUpdated(this);
    }
    return orElse();
  }
}

abstract class StorageEvent_KeyUpdated extends StorageEvent {
  const factory StorageEvent_KeyUpdated({required final String key}) =
      _$StorageEvent_KeyUpdatedImpl;
  const StorageEvent_KeyUpdated._() : super._();

  String get key;

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  _$$StorageEvent_KeyUpdatedImplCopyWith<_$StorageEvent_KeyUpdatedImpl>
  get copyWith => throw _privateConstructorUsedError;
}

/// @nodoc
abstract class _$$StorageEvent_KeyDeletedImplCopyWith<$Res> {
  factory _$$StorageEvent_KeyDeletedImplCopyWith(
    _$StorageEvent_KeyDeletedImpl value,
    $Res Function(_$StorageEvent_KeyDeletedImpl) then,
  ) = __$$StorageEvent_KeyDeletedImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String key});
}

/// @nodoc
class __$$StorageEvent_KeyDeletedImplCopyWithImpl<$Res>
    extends _$StorageEventCopyWithImpl<$Res, _$StorageEvent_KeyDeletedImpl>
    implements _$$StorageEvent_KeyDeletedImplCopyWith<$Res> {
  __$$StorageEvent_KeyDeletedImplCopyWithImpl(
    _$StorageEvent_KeyDeletedImpl _value,
    $Res Function(_$StorageEvent_KeyDeletedImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({Object? key = null}) {
    return _then(
      _$StorageEvent_KeyDeletedImpl(
        key:
            null == key
                ? _value.key
                : key // ignore: cast_nullable_to_non_nullable
                    as String,
      ),
    );
  }
}

/// @nodoc

class _$StorageEvent_KeyDeletedImpl extends StorageEvent_KeyDeleted {
  const _$StorageEvent_KeyDeletedImpl({required this.key}) : super._();

  @override
  final String key;

  @override
  String toString() {
    return 'StorageEvent.keyDeleted(key: $key)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$StorageEvent_KeyDeletedImpl &&
            (identical(other.key, key) || other.key == key));
  }

  @override
  int get hashCode => Object.hash(runtimeType, key);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  @pragma('vm:prefer-inline')
  _$$StorageEvent_KeyDeletedImplCopyWith<_$StorageEvent_KeyDeletedImpl>
  get copyWith => __$$StorageEvent_KeyDeletedImplCopyWithImpl<
    _$StorageEvent_KeyDeletedImpl
  >(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) {
    return keyDeleted(key);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) {
    return keyDeleted?.call(key);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyDeleted != null) {
      return keyDeleted(key);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) {
    return keyDeleted(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) {
    return keyDeleted?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyDeleted != null) {
      return keyDeleted(this);
    }
    return orElse();
  }
}

abstract class StorageEvent_KeyDeleted extends StorageEvent {
  const factory StorageEvent_KeyDeleted({required final String key}) =
      _$StorageEvent_KeyDeletedImpl;
  const StorageEvent_KeyDeleted._() : super._();

  String get key;

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  _$$StorageEvent_KeyDeletedImplCopyWith<_$StorageEvent_KeyDeletedImpl>
  get copyWith => throw _privateConstructorUsedError;
}

/// @nodoc
abstract class _$$StorageEvent_KeyExpiredImplCopyWith<$Res> {
  factory _$$StorageEvent_KeyExpiredImplCopyWith(
    _$StorageEvent_KeyExpiredImpl value,
    $Res Function(_$StorageEvent_KeyExpiredImpl) then,
  ) = __$$StorageEvent_KeyExpiredImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String key});
}

/// @nodoc
class __$$StorageEvent_KeyExpiredImplCopyWithImpl<$Res>
    extends _$StorageEventCopyWithImpl<$Res, _$StorageEvent_KeyExpiredImpl>
    implements _$$StorageEvent_KeyExpiredImplCopyWith<$Res> {
  __$$StorageEvent_KeyExpiredImplCopyWithImpl(
    _$StorageEvent_KeyExpiredImpl _value,
    $Res Function(_$StorageEvent_KeyExpiredImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({Object? key = null}) {
    return _then(
      _$StorageEvent_KeyExpiredImpl(
        key:
            null == key
                ? _value.key
                : key // ignore: cast_nullable_to_non_nullable
                    as String,
      ),
    );
  }
}

/// @nodoc

class _$StorageEvent_KeyExpiredImpl extends StorageEvent_KeyExpired {
  const _$StorageEvent_KeyExpiredImpl({required this.key}) : super._();

  @override
  final String key;

  @override
  String toString() {
    return 'StorageEvent.keyExpired(key: $key)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$StorageEvent_KeyExpiredImpl &&
            (identical(other.key, key) || other.key == key));
  }

  @override
  int get hashCode => Object.hash(runtimeType, key);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  @pragma('vm:prefer-inline')
  _$$StorageEvent_KeyExpiredImplCopyWith<_$StorageEvent_KeyExpiredImpl>
  get copyWith => __$$StorageEvent_KeyExpiredImplCopyWithImpl<
    _$StorageEvent_KeyExpiredImpl
  >(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) {
    return keyExpired(key);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) {
    return keyExpired?.call(key);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyExpired != null) {
      return keyExpired(key);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) {
    return keyExpired(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) {
    return keyExpired?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (keyExpired != null) {
      return keyExpired(this);
    }
    return orElse();
  }
}

abstract class StorageEvent_KeyExpired extends StorageEvent {
  const factory StorageEvent_KeyExpired({required final String key}) =
      _$StorageEvent_KeyExpiredImpl;
  const StorageEvent_KeyExpired._() : super._();

  String get key;

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  _$$StorageEvent_KeyExpiredImplCopyWith<_$StorageEvent_KeyExpiredImpl>
  get copyWith => throw _privateConstructorUsedError;
}

/// @nodoc
abstract class _$$StorageEvent_StorageClearedImplCopyWith<$Res> {
  factory _$$StorageEvent_StorageClearedImplCopyWith(
    _$StorageEvent_StorageClearedImpl value,
    $Res Function(_$StorageEvent_StorageClearedImpl) then,
  ) = __$$StorageEvent_StorageClearedImplCopyWithImpl<$Res>;
}

/// @nodoc
class __$$StorageEvent_StorageClearedImplCopyWithImpl<$Res>
    extends _$StorageEventCopyWithImpl<$Res, _$StorageEvent_StorageClearedImpl>
    implements _$$StorageEvent_StorageClearedImplCopyWith<$Res> {
  __$$StorageEvent_StorageClearedImplCopyWithImpl(
    _$StorageEvent_StorageClearedImpl _value,
    $Res Function(_$StorageEvent_StorageClearedImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
}

/// @nodoc

class _$StorageEvent_StorageClearedImpl extends StorageEvent_StorageCleared {
  const _$StorageEvent_StorageClearedImpl() : super._();

  @override
  String toString() {
    return 'StorageEvent.storageCleared()';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$StorageEvent_StorageClearedImpl);
  }

  @override
  int get hashCode => runtimeType.hashCode;

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) {
    return storageCleared();
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) {
    return storageCleared?.call();
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (storageCleared != null) {
      return storageCleared();
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) {
    return storageCleared(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) {
    return storageCleared?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (storageCleared != null) {
      return storageCleared(this);
    }
    return orElse();
  }
}

abstract class StorageEvent_StorageCleared extends StorageEvent {
  const factory StorageEvent_StorageCleared() =
      _$StorageEvent_StorageClearedImpl;
  const StorageEvent_StorageCleared._() : super._();
}

/// @nodoc
abstract class _$$StorageEvent_CacheInvalidatedImplCopyWith<$Res> {
  factory _$$StorageEvent_CacheInvalidatedImplCopyWith(
    _$StorageEvent_CacheInvalidatedImpl value,
    $Res Function(_$StorageEvent_CacheInvalidatedImpl) then,
  ) = __$$StorageEvent_CacheInvalidatedImplCopyWithImpl<$Res>;
  @useResult
  $Res call({String key});
}

/// @nodoc
class __$$StorageEvent_CacheInvalidatedImplCopyWithImpl<$Res>
    extends
        _$StorageEventCopyWithImpl<$Res, _$StorageEvent_CacheInvalidatedImpl>
    implements _$$StorageEvent_CacheInvalidatedImplCopyWith<$Res> {
  __$$StorageEvent_CacheInvalidatedImplCopyWithImpl(
    _$StorageEvent_CacheInvalidatedImpl _value,
    $Res Function(_$StorageEvent_CacheInvalidatedImpl) _then,
  ) : super(_value, _then);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @pragma('vm:prefer-inline')
  @override
  $Res call({Object? key = null}) {
    return _then(
      _$StorageEvent_CacheInvalidatedImpl(
        key:
            null == key
                ? _value.key
                : key // ignore: cast_nullable_to_non_nullable
                    as String,
      ),
    );
  }
}

/// @nodoc

class _$StorageEvent_CacheInvalidatedImpl
    extends StorageEvent_CacheInvalidated {
  const _$StorageEvent_CacheInvalidatedImpl({required this.key}) : super._();

  @override
  final String key;

  @override
  String toString() {
    return 'StorageEvent.cacheInvalidated(key: $key)';
  }

  @override
  bool operator ==(Object other) {
    return identical(this, other) ||
        (other.runtimeType == runtimeType &&
            other is _$StorageEvent_CacheInvalidatedImpl &&
            (identical(other.key, key) || other.key == key));
  }

  @override
  int get hashCode => Object.hash(runtimeType, key);

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  @override
  @pragma('vm:prefer-inline')
  _$$StorageEvent_CacheInvalidatedImplCopyWith<
    _$StorageEvent_CacheInvalidatedImpl
  >
  get copyWith => __$$StorageEvent_CacheInvalidatedImplCopyWithImpl<
    _$StorageEvent_CacheInvalidatedImpl
  >(this, _$identity);

  @override
  @optionalTypeArgs
  TResult when<TResult extends Object?>({
    required TResult Function(String key) keyCreated,
    required TResult Function(String key) keyUpdated,
    required TResult Function(String key) keyDeleted,
    required TResult Function(String key) keyExpired,
    required TResult Function() storageCleared,
    required TResult Function(String key) cacheInvalidated,
  }) {
    return cacheInvalidated(key);
  }

  @override
  @optionalTypeArgs
  TResult? whenOrNull<TResult extends Object?>({
    TResult? Function(String key)? keyCreated,
    TResult? Function(String key)? keyUpdated,
    TResult? Function(String key)? keyDeleted,
    TResult? Function(String key)? keyExpired,
    TResult? Function()? storageCleared,
    TResult? Function(String key)? cacheInvalidated,
  }) {
    return cacheInvalidated?.call(key);
  }

  @override
  @optionalTypeArgs
  TResult maybeWhen<TResult extends Object?>({
    TResult Function(String key)? keyCreated,
    TResult Function(String key)? keyUpdated,
    TResult Function(String key)? keyDeleted,
    TResult Function(String key)? keyExpired,
    TResult Function()? storageCleared,
    TResult Function(String key)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (cacheInvalidated != null) {
      return cacheInvalidated(key);
    }
    return orElse();
  }

  @override
  @optionalTypeArgs
  TResult map<TResult extends Object?>({
    required TResult Function(StorageEvent_KeyCreated value) keyCreated,
    required TResult Function(StorageEvent_KeyUpdated value) keyUpdated,
    required TResult Function(StorageEvent_KeyDeleted value) keyDeleted,
    required TResult Function(StorageEvent_KeyExpired value) keyExpired,
    required TResult Function(StorageEvent_StorageCleared value) storageCleared,
    required TResult Function(StorageEvent_CacheInvalidated value)
    cacheInvalidated,
  }) {
    return cacheInvalidated(this);
  }

  @override
  @optionalTypeArgs
  TResult? mapOrNull<TResult extends Object?>({
    TResult? Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult? Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult? Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult? Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult? Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult? Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
  }) {
    return cacheInvalidated?.call(this);
  }

  @override
  @optionalTypeArgs
  TResult maybeMap<TResult extends Object?>({
    TResult Function(StorageEvent_KeyCreated value)? keyCreated,
    TResult Function(StorageEvent_KeyUpdated value)? keyUpdated,
    TResult Function(StorageEvent_KeyDeleted value)? keyDeleted,
    TResult Function(StorageEvent_KeyExpired value)? keyExpired,
    TResult Function(StorageEvent_StorageCleared value)? storageCleared,
    TResult Function(StorageEvent_CacheInvalidated value)? cacheInvalidated,
    required TResult orElse(),
  }) {
    if (cacheInvalidated != null) {
      return cacheInvalidated(this);
    }
    return orElse();
  }
}

abstract class StorageEvent_CacheInvalidated extends StorageEvent {
  const factory StorageEvent_CacheInvalidated({required final String key}) =
      _$StorageEvent_CacheInvalidatedImpl;
  const StorageEvent_CacheInvalidated._() : super._();

  String get key;

  /// Create a copy of StorageEvent
  /// with the given fields replaced by the non-null parameter values.
  @JsonKey(includeFromJson: false, includeToJson: false)
  _$$StorageEvent_CacheInvalidatedImplCopyWith<
    _$StorageEvent_CacheInvalidatedImpl
  >
  get copyWith => throw _privateConstructorUsedError;
}
