package transcoder

import (
	"fmt"
	"math"
	"strings"

	"plum/internal/db"
)

func normalizeOpenCLTonemapAlgorithm(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "linear", "gamma", "clip", "reinhard", "hable", "mobius":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "hable"
	}
}

func isHDRTransfer(transfer string) bool {
	t := strings.ToLower(strings.TrimSpace(transfer))
	switch t {
	case "smpte2084", "arib-std-b67", "smpte428_1":
		return true
	default:
		return false
	}
}

func streamSuggestsHDR(s videoStreamInfo) bool {
	if isHDRTransfer(s.ColorTransfer) {
		return true
	}
	pix := strings.ToLower(s.PixelFmt)
	wide := strings.Contains(pix, "10") || strings.Contains(pix, "p010")
	if !wide {
		return false
	}
	prim := strings.ToLower(strings.TrimSpace(s.ColorPrimaries))
	space := strings.ToLower(strings.TrimSpace(s.ColorSpace))
	return prim == "bt2020" || strings.Contains(space, "bt2020")
}

func probeSuggestsHDR(p playbackSourceProbe) bool {
	v, ok := p.primaryVideoStream()
	if !ok {
		return false
	}
	s := videoStreamInfo{
		CodecName:      v.CodecName,
		PixelFmt:       v.PixelFmt,
		ColorPrimaries: v.ColorPrimaries,
		ColorTransfer:  v.ColorTransfer,
		ColorSpace:     v.ColorSpace,
	}
	return streamSuggestsHDR(s)
}

func useOpenCLTonemap(settings db.TranscodingSettings, s videoStreamInfo) bool {
	if !settings.OpenCLToneMappingEnabled {
		return false
	}
	return streamSuggestsHDR(s)
}

func useOpenCLTonemapFromProbe(settings db.TranscodingSettings, p playbackSourceProbe, burnSubtitle bool) bool {
	if burnSubtitle {
		return false
	}
	if !settings.OpenCLToneMappingEnabled {
		return false
	}
	return probeSuggestsHDR(p)
}

func openCLTonemapFilterParams(settings db.TranscodingSettings) string {
	algo := normalizeOpenCLTonemapAlgorithm(settings.OpenCLToneMapAlgorithm)
	desat := settings.OpenCLToneMapDesat
	if math.IsNaN(desat) || math.IsInf(desat, 0) {
		desat = 0.5
	}
	desatStr := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", desat), "0"), ".")
	if desatStr == "" {
		desatStr = "0"
	}
	return fmt.Sprintf(
		"tonemap_opencl=tonemap=%s:desat=%s:t=bt2020:m=bt2020:p=bt2020:format=nv12",
		algo,
		desatStr,
	)
}

func swPixelFormatForOpenCLUpload(s videoStreamInfo) string {
	pix := strings.ToLower(s.PixelFmt)
	if strings.Contains(pix, "p010") || strings.Contains(pix, "10") {
		return "p010le"
	}
	return "nv12"
}

func hwDownloadFormatForOpenCL(s videoStreamInfo) string {
	if isTenBitStream(s) {
		return "p010le"
	}
	return "nv12"
}

func softwareOpenCLTonemapVF(settings db.TranscodingSettings, stream videoStreamInfo) string {
	if !useOpenCLTonemap(settings, stream) {
		return ""
	}
	swf := swPixelFormatForOpenCLUpload(stream)
	return fmt.Sprintf(
		"format=%s,hwupload=derive_device=opencl,%s,hwdownload,format=yuv420p",
		swf,
		openCLTonemapFilterParams(settings),
	)
}

func vaapiPrefixWithOptionalOpenCLTonemap(
	settings db.TranscodingSettings,
	stream videoStreamInfo,
	vaapiDecode bool,
) string {
	if !useOpenCLTonemap(settings, stream) {
		return ""
	}
	dl := hwDownloadFormatForOpenCL(stream)
	tm := openCLTonemapFilterParams(settings)
	if vaapiDecode {
		return fmt.Sprintf("hwdownload,format=%s,hwupload=derive_device=opencl,%s,hwupload=derive_device=vaapi,", dl, tm)
	}
	swf := swPixelFormatForOpenCLUpload(stream)
	return fmt.Sprintf("format=%s,hwupload=derive_device=opencl,%s,hwupload=derive_device=vaapi,", swf, tm)
}
