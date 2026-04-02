package plum.tv.core.data.di

import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import plum.tv.core.data.AuthTokenBridge

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {
    @Provides
    @Singleton
    fun providePlumOkHttpClient(bridge: AuthTokenBridge): OkHttpClient {
        val auth = Interceptor { chain ->
            val token = bridge.bearerToken()
            val req =
                if (token.isNullOrEmpty()) {
                    chain.request()
                } else {
                    chain.request().newBuilder().header("Authorization", "Bearer $token").build()
                }
            chain.proceed(req)
        }
        val logging = HttpLoggingInterceptor().apply { level = HttpLoggingInterceptor.Level.BASIC }
        return OkHttpClient.Builder()
            .addInterceptor(auth)
            .addInterceptor(logging)
            .build()
    }
}
