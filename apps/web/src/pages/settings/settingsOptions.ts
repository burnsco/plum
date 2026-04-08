import type {
  HardwareEncodeFormat,
  MetadataArtworkSettings as MetadataArtworkSettingsShape,
  OpenCLToneMapAlgorithm,
  VaapiDecodeCodec,
} from "@plum/contracts";

export const decodeCodecOptions: Array<{
  key: VaapiDecodeCodec;
  label: string;
  description: string;
}> = [
  { key: "h264", label: "H.264", description: "Use VAAPI decode for 8-bit AVC video." },
  { key: "hevc", label: "HEVC", description: "Use VAAPI decode for standard HEVC streams." },
  { key: "mpeg2", label: "MPEG-2", description: "Use VAAPI decode for legacy MPEG-2 sources." },
  { key: "vc1", label: "VC-1", description: "Use VAAPI decode for VC-1 content when available." },
  { key: "vp8", label: "VP8", description: "Use VAAPI decode for VP8 sources." },
  { key: "vp9", label: "VP9", description: "Use VAAPI decode for standard VP9 streams." },
  { key: "av1", label: "AV1", description: "Use VAAPI decode for AV1 content." },
  {
    key: "hevc10bit",
    label: "HEVC 10-bit",
    description: "Allow VAAPI decode for 10-bit HEVC video.",
  },
  { key: "vp910bit", label: "VP9 10-bit", description: "Allow VAAPI decode for 10-bit VP9 video." },
];

export const openclTonemapAlgorithmOptions: Array<{
  value: OpenCLToneMapAlgorithm;
  label: string;
  description: string;
}> = [
  {
    value: "hable",
    label: "Hable",
    description: "Filmic curve; a common default for HDR fiction and games.",
  },
  {
    value: "reinhard",
    label: "Reinhard",
    description: "Smooth rolloff; can look softer on very bright highlights.",
  },
  {
    value: "mobius",
    label: "Mobius",
    description: "Preserves highlights with a gentle knee.",
  },
  {
    value: "linear",
    label: "Linear",
    description: "Simple linear stretch; can clip or look harsh on strong HDR.",
  },
  {
    value: "gamma",
    label: "Gamma",
    description: "Power-law compression; fast but less perceptually tuned.",
  },
  {
    value: "clip",
    label: "Clip",
    description: "Hard clip to SDR range; mostly useful as a baseline comparison.",
  },
];

export const encodeFormatOptions: Array<{
  key: HardwareEncodeFormat;
  label: string;
  description: string;
}> = [
  { key: "h264", label: "H.264", description: "Best playback compatibility and safest default." },
  {
    key: "hevc",
    label: "HEVC",
    description: "Smaller output with newer client support requirements.",
  },
  {
    key: "av1",
    label: "AV1",
    description: "Highest efficiency, but hardware support varies widely.",
  },
];

export const movieArtworkProviderOptions: Array<{
  key: keyof MetadataArtworkSettingsShape["movies"];
  label: string;
  description: string;
}> = [
  { key: "fanart", label: "Fanart", description: "Use fanart.tv artwork when it is available." },
  { key: "tmdb", label: "TMDB", description: "Use TMDB posters for movies and series." },
  { key: "tvdb", label: "TVDB", description: "Use TVDB posters for movies and series." },
];

export const showArtworkProviderOptions: Array<{
  key: keyof MetadataArtworkSettingsShape["shows"];
  label: string;
  description: string;
}> = [
  { key: "fanart", label: "Fanart", description: "Use fanart.tv artwork when it is available." },
  { key: "tmdb", label: "TMDB", description: "Use TMDB posters for shows and seasons." },
  { key: "tvdb", label: "TVDB", description: "Use TVDB posters for shows and seasons." },
];

export const episodeArtworkProviderOptions: Array<{
  key: keyof MetadataArtworkSettingsShape["episodes"];
  label: string;
  description: string;
}> = [
  { key: "tmdb", label: "TMDB", description: "Use TMDB stills and episode posters first." },
  { key: "tvdb", label: "TVDB", description: "Use TVDB episode artwork when available." },
  { key: "omdb", label: "OMDb", description: "Use OMDb when an episode IMDb ID is known." },
];
