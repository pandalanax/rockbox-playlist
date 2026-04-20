These are the systemd unit files used for the ROCK 2F `rockbox-playlist` setup.

Files:

- `rockbox-playlist.service`: SSH app server on port `2222`
- `rockbox-playlist-autosync.service`: mount-triggered autosync with a 5-second grace period
- `rockbox-playlist-led-sos.service`: persistent SOS LED alarm on autosync failure

The autosync unit assumes:

- player mount: `/media/pandalanax/4B03-0C07`
- sync source: `/media/pandalanax/Player`
- config lives under `/home/pandalanax/.config/rockbox-playlist`

These files document the current deployed setup and can be copied into `/etc/systemd/system/` on the Pi.
