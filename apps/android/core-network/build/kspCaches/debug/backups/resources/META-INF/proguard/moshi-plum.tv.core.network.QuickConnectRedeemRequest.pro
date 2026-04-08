-keepnames class plum.tv.core.network.QuickConnectRedeemRequest
-if class plum.tv.core.network.QuickConnectRedeemRequest
-keep class plum.tv.core.network.QuickConnectRedeemRequestJsonAdapter {
    public <init>(com.squareup.moshi.Moshi);
}
