#!/usr/bin/env -S gjs -m

import Cairo from "cairo";
import Adw from "gi://Adw?version=1";
import GLib from "gi://GLib";
import GObject from "gi://GObject";
import Gio from "gi://Gio";
import Gtk from "gi://Gtk?version=4.0";
import system from "system";

const resource = Gio.Resource.load(
  GLib.build_filenamev([
    GLib.get_current_dir(),
    "assets/resources/index.gresource",
  ])
);
Gio.resources_register(resource);

const  _ = (text) => GLib.dgettext("default", text)

const SessionsMainWindow = GObject.registerClass(
  {
    GTypeName: "SessionsMainWindow",
    Template: "resource:///com/pojtinger/felicitas/Sessions/window.ui",
    InternalChildren: [
      "dial_area",
      "analog_time_label",
      "action_button",
      "plus_button",
      "minus_button",
    ],
  },
  class SessionsMainWindow extends Adw.ApplicationWindow {
    #totalSec = 300;
    #running = false;
    #remain = 0;
    #timer = 0;
    #dragging = false;
    #paused = false;
    #alarmMediaFile = null;

    constructor(params) {
      super(params);
      this.#setupAlarm();
      this.#setupDialDrawing();
      this.#setupDialGestures();
    }

    #setupAlarm() {
      this.#alarmMediaFile = Gtk.MediaFile.new_for_resource(
        "/com/pojtinger/felicitas/Sessions/alarm-clock-elapsed.oga"
      );
    }

    startAlarmPlayback() {
      this.#alarmMediaFile.seek(0);
      this.#alarmMediaFile.play();
    }

    stopAlarmPlayback() {
      this.#alarmMediaFile.set_playing(false);
      this.#alarmMediaFile.seek(0);
    }

    updateButtons() {
      if (this.#running) {
        this._action_button.set_icon_name("media-playback-stop-symbolic");
        this._action_button.set_label(_("Stop"));
        this._action_button.remove_css_class("suggested-action");
        this._action_button.add_css_class("destructive-action");
      } else {
        this._action_button.set_icon_name("media-playback-start-symbolic");
        this._action_button.set_label(_("Start Timer"));
        this._action_button.remove_css_class("destructive-action");
        this._action_button.add_css_class("suggested-action");
      }

      this._plus_button.set_sensitive(this.#totalSec < 3600);
      this._minus_button.set_sensitive(this.#totalSec > 30);
    }

    updateDial() {
      let m, s;
      if (this.#running) {
        m = Math.floor(this.#remain / 60);
        s = this.#remain % 60;
      } else {
        m = Math.floor(this.#totalSec / 60);
        s = this.#totalSec % 60;
      }

      this._analog_time_label.set_text(
        `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`
      );
      this._dial_area.queue_draw();
    }

    #createSessionFinishedHandler() {
      return () => {
        if (!this.#running) {
          return GLib.SOURCE_REMOVE;
        }

        this.#remain--;
        this.updateDial();

        if (this.#remain <= 0) {
          this.#running = false;
          if (this.#timer > 0) {
            GLib.Source.remove(this.#timer);
            this.#timer = 0;
          }

          this.updateButtons();
          this.updateDial();

          this.startAlarmPlayback();

          const notification = Gio.Notification.new(_("Session finished"));
          notification.set_priority(Gio.NotificationPriority.URGENT);
          notification.set_default_action("app.stopAlarmPlayback");
          notification.add_button(_("Stop alarm"), "app.stopAlarmPlayback");

          this.application.send_notification("session-finished", notification);

          return GLib.SOURCE_REMOVE;
        }

        return GLib.SOURCE_CONTINUE;
      };
    }

    startTimer() {
      this.stopAlarmPlayback();

      this.#running = true;
      this.#remain = this.#totalSec;

      this.updateButtons();
      this.updateDial();

      this.#timer = GLib.timeout_add(
        GLib.PRIORITY_DEFAULT,
        1000,
        this.#createSessionFinishedHandler()
      );
    }

    stopTimer() {
      this.#running = false;
      if (this.#timer > 0) {
        GLib.Source.remove(this.#timer);
        this.#timer = 0;
      }

      this.updateButtons();
      this.updateDial();
    }

    #resumeTimer() {
      if (this.#remain > 0) {
        this.#timer = GLib.timeout_add(
          GLib.PRIORITY_DEFAULT,
          1000,
          this.#createSessionFinishedHandler()
        );
      }
    }

    #handleDialing(x, y) {
      if (this.#running && !this.#dragging) {
        this.#paused = true;
        if (this.#timer > 0) {
          GLib.Source.remove(this.#timer);
          this.#timer = 0;
        }
      }

      const w = this._dial_area.get_width();
      const h = this._dial_area.get_height();
      const cx = w / 2;
      const cy = h / 2;
      const dx = x - cx;
      const dy = y - cy;

      if (Math.sqrt(dx * dx + dy * dy) < 15) {
        return;
      }

      let a = Math.atan2(dy, dx) + Math.PI / 2;
      if (a < 0) {
        a += 2 * Math.PI;
      }

      let intervals = Math.floor((a / (2 * Math.PI)) * 120);
      if (intervals === 0) {
        intervals = 120;
      }

      this.#totalSec = intervals * 30;

      if (this.#paused) {
        this.#remain = this.#totalSec;
      }

      this.updateDial();
    }

    #setupDialDrawing() {
      this._dial_area.set_draw_func((area, cr, w, h) => {
        const cx = w / 2;
        const cy = h / 2;
        const r = Math.min(cx, cy) - 15;

        const style = area.get_style_context();
        const [, accent] = style.lookup_color("accent_bg_color");
        const [, err] = style.lookup_color("error_bg_color");

        cr.setSourceRGB(0.7, 0.7, 0.7);
        cr.setLineWidth(10);
        cr.arc(cx, cy, r, 0, 2 * Math.PI);
        cr.stroke();

        if (this.#totalSec > 0) {
          const progress = this.#totalSec / 3600.0;
          const end = -Math.PI / 2 + 2 * Math.PI * progress;
          let handleAngle,
            handleColor,
            angle,
            lineColor,
            fillR,
            fillG,
            fillB,
            fillA;

          if (this.#running && this.#remain > 0) {
            const ratio = this.#remain / this.#totalSec;
            angle = -Math.PI / 2 + 2 * Math.PI * progress * ratio;
            lineColor = err;
            fillR = err.red;
            fillG = err.green;
            fillB = err.blue;
            fillA = 0.3;
          } else {
            angle = end;
            lineColor = accent;
            fillR = 0.6;
            fillG = 0.6;
            fillB = 0.6;
            fillA = 0.2;
          }

          cr.setSourceRGBA(fillR, fillG, fillB, fillA);
          cr.moveTo(cx, cy);
          cr.arc(cx, cy, r, -Math.PI / 2, angle);
          cr.lineTo(cx, cy);
          cr.fill();

          cr.setSourceRGB(lineColor.red, lineColor.green, lineColor.blue);
          cr.setLineWidth(10);
          cr.setLineCap(Cairo.LineCap.ROUND);
          cr.arc(cx, cy, r, -Math.PI / 2, angle);
          cr.stroke();

          handleAngle = angle;
          handleColor = lineColor;

          const dx = cx + r * Math.cos(handleAngle);
          const dy = cy + r * Math.sin(handleAngle);
          cr.setSourceRGB(handleColor.red, handleColor.green, handleColor.blue);
          cr.save();
          cr.translate(dx, dy);
          cr.arc(0, 0, 8, 0, 2 * Math.PI);
          cr.fill();
          cr.restore();
        }
      });
    }

    #setupDialGestures() {
      const drag = Gtk.GestureDrag.new();
      drag.connect("drag-begin", (_gesture, x, y) => {
        this.#dragging = true;
        this.#handleDialing(x, y);
      });
      drag.connect("drag-update", (gesture, dx, dy) => {
        if (this.#dragging) {
          const [, x, y] = gesture.get_start_point();
          this.#handleDialing(x + dx, y + dy);
        }
      });
      drag.connect("drag-end", (_gesture, _dx, _dy) => {
        this.#dragging = false;

        if (this.#paused) {
          this.#paused = false;
          this.#resumeTimer();
        } else if (!this.#running && this.#totalSec > 0) {
          this.startTimer();
        }
      });

      const click = Gtk.GestureClick.new();
      click.connect("pressed", (_gesture, _n, x, y) => {
        this.#handleDialing(x, y);
      });

      this._dial_area.add_controller(drag);
      this._dial_area.add_controller(click);
    }

    addTime() {
      if (this.#totalSec < 3600) {
        this.#totalSec += 30;
        if (this.#running) {
          this.#remain = this.#totalSec;
        }

        this.updateDial();
        this.updateButtons();
      }
    }

    removeTime() {
      if (this.#totalSec > 30) {
        this.#totalSec -= 30;
        if (this.#running) {
          this.#remain = this.#totalSec;
        }

        this.updateDial();
        this.updateButtons();
      }
    }

    toggleTimer() {
      if (this.#running) {
        this.stopTimer();
      } else if (this.#totalSec > 0) {
        this.startTimer();
      }
    }
  }
);

const SessionsApplication = GObject.registerClass(
  {
    GTypeName: "SessionsApplication",
  },
  class SessionsApplication extends Adw.Application {
    constructor() {
      super({
        application_id: "com.pojtinger.felicitas.Sessions",
        flags: Gio.ApplicationFlags.DEFAULT_FLAGS,
      });
    }

    #window = null;
    #aboutDialog = null;

    vfunc_activate() {
      if (this.#window !== null) {
        this.#window.present();
        return;
      }

      this.#setupAboutDialog();
      this.#setupWindow();
      this.#setupActions();

      this.add_window(this.#window);
      this.#window.present();
    }

    #setupAboutDialog() {
      this.#aboutDialog = Adw.AboutDialog.new_from_appdata(
        "/com/pojtinger/felicitas/Sessions/metainfo.xml",
        "0.1.6"
      );
    }

    #setupWindow() {
      this.#window = new SessionsMainWindow({
        application: this,
      });

      this.#window.updateButtons();
      this.#window.updateDial();
    }

    #setupActions() {
      const toggleTimerAction = Gio.SimpleAction.new("toggleTimer", null);
      toggleTimerAction.connect("activate", () => {
        this.#window.toggleTimer();
      });
      this.add_action(toggleTimerAction);

      const addTimeAction = Gio.SimpleAction.new("addTime", null);
      addTimeAction.connect("activate", () => {
        this.#window.addTime();
      });
      this.add_action(addTimeAction);

      const removeTimeAction = Gio.SimpleAction.new("removeTime", null);
      removeTimeAction.connect("activate", () => {
        this.#window.removeTime();
      });
      this.add_action(removeTimeAction);

      const openAboutAction = Gio.SimpleAction.new("openAbout", null);
      openAboutAction.connect("activate", () => {
        this.#aboutDialog.present(this.#window);
      });
      this.add_action(openAboutAction);

      const stopAlarmPlaybackAction = Gio.SimpleAction.new(
        "stopAlarmPlayback",
        null
      );
      stopAlarmPlaybackAction.connect("activate", () => {
        this.#window.stopAlarmPlayback();
        this.activate();
      });
      this.add_action(stopAlarmPlaybackAction);
    }
  }
);

new SessionsApplication().run([system.programInvocationName, ...ARGV]);
