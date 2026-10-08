"""Finder layout shared by local packages and signed release packages."""

from pathlib import Path

app = Path(defines["app"]).resolve()
files = [str(app)]
symlinks = {"Applications": "/Applications"}
icon = str(app / "Contents/Resources/Backlog.icns")
background = defines["background"]
format = "UDZO"
filesystem = "HFS+"
window_rect = ((200, 160), (720, 460))
default_view = "icon-view"
show_toolbar = False
show_sidebar = False
show_status_bar = False
show_pathbar = False
show_tab_view = False
arrange_by = None
icon_size = 96
text_size = 14
label_pos = "bottom"
icon_locations = {app.name: (190, 244), "Applications": (530, 244)}
hide_extensions = [app.name]
