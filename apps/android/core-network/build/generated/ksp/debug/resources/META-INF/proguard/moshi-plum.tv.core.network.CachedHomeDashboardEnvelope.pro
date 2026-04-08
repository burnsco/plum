-keepnames class plum.tv.core.network.CachedHomeDashboardEnvelope
-if class plum.tv.core.network.CachedHomeDashboardEnvelope
-keep class plum.tv.core.network.CachedHomeDashboardEnvelopeJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
