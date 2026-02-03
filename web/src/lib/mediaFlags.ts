// Media flag icon mappings (icons from Tautulli, GPL-3.0)

export function getAudioCodecIcon(codec?: string): string | null {
  if (!codec) return null
  const c = codec.toLowerCase()

  if (c.includes('truehd')) return '/media-flags/audio_codec/dolby_truehd.png'
  if (c === 'eac3' || c.includes('ddp') || c.includes('dd+')) return '/media-flags/audio_codec/eac3.png'
  if (c === 'ac3' || c === 'dd' || c.includes('dolby')) return '/media-flags/audio_codec/dolby_digital.png'
  if (c.includes('dts-hd') || c.includes('dts:x') || c === 'dca-ma') return '/media-flags/audio_codec/dca-ma.png'
  if (c.includes('dts')) return '/media-flags/audio_codec/dts.png'
  if (c === 'aac') return '/media-flags/audio_codec/aac.png'
  if (c === 'flac') return '/media-flags/audio_codec/flac.png'
  if (c === 'mp3') return '/media-flags/audio_codec/mp3.png'
  if (c === 'pcm' || c === 'lpcm') return '/media-flags/audio_codec/pcm.png'

  return null
}

export function getVideoCodecIcon(codec?: string): string | null {
  if (!codec) return null
  const c = codec.toLowerCase()

  if (c === 'hevc' || c === 'h265') return '/media-flags/video_codec/hevc.png'
  if (c === 'h264' || c === 'avc') return '/media-flags/video_codec/h264.png'
  if (c === 'vc1') return '/media-flags/video_codec/vc1.png'

  return null
}

export function getResolutionIcon(resolution?: string): string | null {
  if (!resolution) return null
  const r = resolution.toLowerCase().replace('p', '').replace('i', '')

  if (r === '2160' || r === '4k' || r === 'uhd') return '/media-flags/video_resolution/4k.png'
  if (r === '1080') return '/media-flags/video_resolution/1080.png'
  if (r === '720') return '/media-flags/video_resolution/720.png'
  if (r === '480' || r === '576') return '/media-flags/video_resolution/480.png'
  if (parseInt(r) < 480) return '/media-flags/video_resolution/sd.png'

  return null
}

export function getChannelsIcon(channels?: number): string | null {
  if (!channels) return null

  if (channels === 8) return '/media-flags/audio_channels/8.png'
  if (channels >= 6) return '/media-flags/audio_channels/6.png'
  if (channels === 2) return '/media-flags/audio_channels/2.png'

  return null
}
