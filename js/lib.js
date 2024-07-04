function rpc(input, callbackFn) {
  var x = new XMLHttpRequest();
  x.open('POST', 'rpc', true);
  x.setRequestHeader('Content-type', 'application/x-www-form-urlencoded');
  x.onreadystatechange = function() {
    if (x.readyState != 4) {
      return;
    }
    if (x.status != 200) {
      callbackFn(input, {Error: "Server and client connection lost!"});
      eStop = true;
      return;
    }
    var raw;
    try {
      raw = JSON.parse(x.responseText);
    } catch (ex) {
      callbackFn(input, {
        Error: 'bad JSON (' + ex + '):' + x.responseText
      });
      return;
    }
    if (raw.Error) {
      callbackFn(input, {Error: raw.Error});
      return;
    }
    callbackFn(input, raw);
  };
  x.send('rpc=' + JSON.stringify(input));
}

var p;

function render(xs, ys) {
  let pts = [];
  for (var i=0; i<xs.length && i<ys.length; i++) {
    pts.push([xs[i]/10, ys[i]/10]);
  }
  p.Line(pts);
  p.Frame();
}

var scale;
var samples;

function callback(req, resp) {
  if (req == null) {
    return;
  }
  if (resp.Error != '') {
    console.log("got error: " + resp.Error);
    return;
  }
  switch (req.Cmd) {
  case "scale":
    scale = resp.Values;
    break;
  case "sample":
    samples = resp.Values;
    break;
  default:
    console.log("no idea about:" + req.Cmd)
  }
  if (scale != null && samples != null) {
    console.log("got scale:"+scale.length+" got samples:"+samples.length);
    render(scale, samples);
  }
}

// This function was inspired by the FORTRAN code here:
// https://www.physics.sfasu.edu/astro/color/spectra.html
// We use a larger gamma value.
function renderColors() {
  let bx = (p.CoordMaxX-p.CoordMinX)/(p.MaxX-p.MinX);
  let ax = p.CoordMaxX - bx*p.MaxX;
  let gamma = 0.96;
  let oldStyle = p.attr.ctx.strokeStyle;
  for (var px = p.MinX; px < p.MaxX; px+=1) {
    let nm = ax + bx*px;
    let r=0, g=0, b=0;
    if (nm < 380 || nm > 780) {
      // default to black
    } else if (nm <= 440) {
      r = (440-nm)/(440-380);
      b = 1;
    } else if (nm <= 490) {
      g = (nm-440)/(490-440);
      b = 1;
    } else if (nm <= 510) {
      g = 1;
      b = (510-nm)/(510-490);
    } else if (nm <= 580) {
      r = (nm-510)/(580-510);
      g = 1;
    } else if (nm <= 645) {
      r = 1;
      g = (645-nm)/(645-580);
    } else {
      r = 1;
    }
    let mag = 255;
    if (nm < 380 || nm > 780) {
      mag = 0;
    } else if (nm > 700) {
      mag = 255*(.3 + .7*(780-nm)/(780-700));
    } else if (nm < 420) {
      mag = 255*(.3 + .7*(nm-380)/(420-380));
    }
    let R = r ? Math.round(Math.pow(mag*r, gamma)) : 0;
    let G = g ? Math.round(Math.pow(mag*g, gamma)) : 0;
    let B = b ? Math.round(Math.pow(mag*b, gamma)) : 0;
    p.attr.ctx.strokeStyle = "rgb(" + R + "," + G + "," + B + ")";
    p.attr.ctx.beginPath();
    p.attr.ctx.moveTo(px, p.MinY + 30);
    p.attr.ctx.lineTo(px, p.MinY + 50);
    p.attr.ctx.stroke();
  }
  p.attr.ctx.strokeStyle = oldStyle;
}

function start(pl) {
  p = pl;
  // Open up some space for the color block
  p.MinY -= 50;
  p.Frame();

  p.CoordMaxX = 900;
  p.CoordMinX = 200;
  p.CoordMinY = 0;
  p.CoordMaxY = 500;
  var xts = [
    [200, true, '200nm'],
    [300, true, '300nm'],
    [400, true, '400nm'],
    [500, true, '500nm'],
    [600, true, '600nm'],
    [700, true, '700nm'],
    [800, true, '800nm'],
    [900, true, '900nm']
 ];
  var yts = [
    [0, true, '0'],
    [50, false, '50'],
    [100, true, '100'],
    [150, false, '150'],
    [200, true, '200'],
    [250, false, '250'],
    [300, true, '300'],
    [350, false, '350'],
    [400, true, '400'],
    [450, false, '450'],
    [500, true, '500']
  ];
  p.Axis(ZappemNet.Plotter.X0, xts);
  p.Axis(ZappemNet.Plotter.X1, xts);
  p.Axis(ZappemNet.Plotter.Y0, yts);
  p.Axis(ZappemNet.Plotter.Y1, yts);
  renderColors();
  rpc({Cmd: 'scale'}, callback);
  rpc({Cmd: 'sample'}, callback);
}
