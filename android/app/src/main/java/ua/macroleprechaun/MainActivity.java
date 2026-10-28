package ua.macroleprechaun;

import android.app.Activity;
import android.os.Bundle;
import android.content.SharedPreferences;
import android.graphics.Color;
import android.net.nsd.NsdManager;
import android.net.nsd.NsdServiceInfo;
import android.net.wifi.WifiManager;
import android.view.View;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.WebResourceRequest;
import android.widget.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import org.json.JSONObject;

public class MainActivity extends Activity {
 private final ExecutorService executor=Executors.newFixedThreadPool(2);
 private WebView web;
 private EditText address;
 private TextView status;
 private LinearLayout setup;
 private SharedPreferences prefs;
 private String origin="";
 private volatile boolean destroyed=false;
 private NsdManager nsd;
 private NsdManager.DiscoveryListener discovery;
 private WifiManager.MulticastLock lock;
 private boolean resolving=false;
 private boolean english;
 private JSONObject strings=new JSONObject();
 private String tr(String key){return strings.optString(key,key);}
 private void loadLocale(){try(java.io.InputStream in=getAssets().open("locales/"+(english?"en":"uk")+".json")){java.io.ByteArrayOutputStream out=new java.io.ByteArrayOutputStream();byte[] b=new byte[4096];int n;while((n=in.read(b))!=-1)out.write(b,0,n);strings=new JSONObject(out.toString("UTF-8"));}catch(Exception ignored){}}
 @Override public void onCreate(Bundle b){super.onCreate(b);english=java.util.Locale.getDefault().getLanguage().equals("en");loadLocale();prefs=getSharedPreferences("connection",MODE_PRIVATE);showSetup();}
 private int dp(int x){return (int)(getResources().getDisplayMetrics().density*x);}
 private void showSetup(){getWindow().getDecorView().setSystemUiVisibility(0);stopDiscovery();if(web!=null){web.destroy();web=null;}setup=new LinearLayout(this);setup.setOrientation(LinearLayout.VERTICAL);setup.setPadding(dp(26),dp(42),dp(26),dp(20));setup.setBackgroundColor(Color.rgb(16,18,22));TextView title=new TextView(this);title.setText("✳ MacroLeprechaun");title.setTextSize(25);title.setTextColor(Color.rgb(109,222,145));setup.addView(title);TextView info=new TextView(this);info.setText(tr("s0"));info.setTextColor(Color.LTGRAY);info.setPadding(0,dp(25),0,dp(16));setup.addView(info);address=new EditText(this);address.setSingleLine(true);address.setTextColor(Color.WHITE);address.setText(prefs.getString("address","http://macroleprechaun.local:47640"));address.setInputType(17);setup.addView(address);Button connect=new Button(this);connect.setText(tr("s1"));setup.addView(connect);connect.setOnClickListener(v->connect());Button discover=new Button(this);discover.setText(tr("s2"));setup.addView(discover);discover.setOnClickListener(v->discover());status=new TextView(this);status.setTextColor(Color.LTGRAY);status.setPadding(0,dp(18),0,0);setup.addView(status);setContentView(setup);}
 private void connect(){String raw=address.getText().toString().trim();if(!raw.startsWith("http://"))raw="http://"+raw;try{URI u=new URI(raw);if(!"http".equals(u.getScheme())||u.getHost()==null||u.getUserInfo()!=null||u.getFragment()!=null||u.getQuery()!=null)throw new Exception();origin="http://"+u.getHost()+":"+(u.getPort()<0?47640:u.getPort());}catch(Exception e){status.setText(tr("s3"));return;}status.setText(tr("s4"));final String base=origin;executor.execute(()->{try{HttpURLConnection c=(HttpURLConnection)new URL(base+"/api/discover").openConnection();c.setConnectTimeout(4000);c.setReadTimeout(4000);try{java.io.ByteArrayOutputStream out=new java.io.ByteArrayOutputStream();byte[] buffer=new byte[4096];int count;try(java.io.InputStream in=c.getInputStream()){while((count=in.read(buffer))!=-1){out.write(buffer,0,count);if(out.size()>16384)throw new Exception("Response too large");}}String text=new String(out.toByteArray(),StandardCharsets.UTF_8);if(!"MacroLeprechaun".equals(new JSONObject(text).getString("name")))throw new Exception("Wrong server");}finally{c.disconnect();}runOnUiThread(()->{if(!destroyed){prefs.edit().putString("address",base).apply();openPanel(base);}});}catch(Exception e){runOnUiThread(()->{if(!destroyed)status.setText(tr("s5"));});}});}
 private void openPanel(String base){getWindow().getDecorView().setSystemUiVisibility(5894);stopDiscovery();web=new WebView(this);web.setVerticalScrollBarEnabled(false);web.setHorizontalScrollBarEnabled(false);web.setOverScrollMode(View.OVER_SCROLL_NEVER);web.setBackgroundColor(Color.rgb(16,18,22));web.getSettings().setCacheMode(android.webkit.WebSettings.LOAD_NO_CACHE);web.clearCache(true);web.getSettings().setJavaScriptEnabled(true);web.getSettings().setDomStorageEnabled(true);web.getSettings().setAllowFileAccess(false);web.getSettings().setAllowContentAccess(false);web.setWebViewClient(new WebViewClient(){@Override public boolean shouldOverrideUrlLoading(WebView view,WebResourceRequest req){try{return !sameOrigin(req.getUrl().toString(),base);}catch(Exception e){return true;}}});setContentView(web);web.loadUrl(base+"/remote.html");}
 private boolean sameOrigin(String value,String base)throws Exception{URI a=new URI(value),b=new URI(base);return b.getScheme().equals(a.getScheme())&&b.getHost().equals(a.getHost())&&b.getPort()==a.getPort();}
 private void found(String host,int port){runOnUiThread(()->{if(destroyed||web!=null)return;address.setText("http://"+host+":"+port);status.setText(tr("s6"));});}
 private void discover(){stopDiscovery();status.setText(tr("s7"));try{WifiManager wifi=(WifiManager)getApplicationContext().getSystemService(WIFI_SERVICE);lock=wifi.createMulticastLock("MacroLeprechaunDiscovery");lock.setReferenceCounted(false);lock.acquire();nsd=(NsdManager)getSystemService(NSD_SERVICE);discovery=new NsdManager.DiscoveryListener(){public void onDiscoveryStarted(String s){}public void onDiscoveryStopped(String s){}public void onStartDiscoveryFailed(String s,int e){}public void onStopDiscoveryFailed(String s,int e){}public void onServiceLost(NsdServiceInfo i){}public void onServiceFound(NsdServiceInfo i){if(resolving)return;resolving=true;nsd.resolveService(i,new NsdManager.ResolveListener(){public void onResolveFailed(NsdServiceInfo i,int e){resolving=false;}public void onServiceResolved(NsdServiceInfo i){resolving=false;found(i.getHost().getHostAddress(),i.getPort());}});}};nsd.discoverServices("_macroleprechaun._tcp.",NsdManager.PROTOCOL_DNS_SD,discovery);}catch(Exception ignored){}
 executor.execute(()->{try(DatagramSocket s=new DatagramSocket()){s.setBroadcast(true);s.setSoTimeout(4000);byte[] query="MACROLEPRECHAUN_DISCOVER_V1".getBytes(StandardCharsets.UTF_8);s.send(new DatagramPacket(query,query.length,InetAddress.getByName("255.255.255.255"),47641));byte[] b=new byte[2048];DatagramPacket reply=new DatagramPacket(b,b.length);s.receive(reply);JSONObject o=new JSONObject(new String(b,0,reply.getLength(),StandardCharsets.UTF_8));if("MacroLeprechaun".equals(o.getString("name")))found(reply.getAddress().getHostAddress(),o.getInt("port"));}catch(Exception ignored){}});
 setup.postDelayed(()->{if(!destroyed)stopDiscovery();},12000);
 }
 private void stopDiscovery(){if(nsd!=null&&discovery!=null){try{nsd.stopServiceDiscovery(discovery);}catch(Exception ignored){}discovery=null;}if(lock!=null&&lock.isHeld())lock.release();resolving=false;}
 @Override public void onBackPressed(){if(web!=null)showSetup();else super.onBackPressed();}
 @Override protected void onDestroy(){destroyed=true;stopDiscovery();if(web!=null)web.destroy();executor.shutdownNow();super.onDestroy();}
}
