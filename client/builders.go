package client

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"time"

	"github.com/Mrs4s/MiraiGo/binary"
	"github.com/Mrs4s/MiraiGo/binary/jce"
	"github.com/Mrs4s/MiraiGo/client/internal/network"
	"github.com/Mrs4s/MiraiGo/client/internal/oicq"
	"github.com/Mrs4s/MiraiGo/client/pb"
	"github.com/Mrs4s/MiraiGo/client/pb/cmd0x352"
	"github.com/Mrs4s/MiraiGo/client/pb/msg"
	"github.com/Mrs4s/MiraiGo/client/pb/oidb"
	"github.com/Mrs4s/MiraiGo/client/pb/profilecard"
	"github.com/Mrs4s/MiraiGo/client/pb/structmsg"
	"github.com/Mrs4s/MiraiGo/internal/proto"
	"github.com/Mrs4s/MiraiGo/internal/tlv"
	"github.com/Mrs4s/MiraiGo/wrapper"
)

var (
	syncConst1 = rand.Int63()
	syncConst2 = rand.Int63()
)

func buildCode2DRequestPacket(seq uint32, j uint64, cmd uint16, bodyFunc func(writer *binary.Writer)) []byte {
	return binary.NewWriterF(func(w *binary.Writer) {
		w.WriteByte(2)
		pos := w.FillUInt16()
		w.WriteUInt16(cmd)
		w.Write(make([]byte, 21))
		w.WriteByte(3)
		w.WriteUInt16(0)
		w.WriteUInt16(50) // version
		w.WriteUInt32(seq)
		w.WriteUInt64(j)
		bodyFunc(w)
		w.WriteByte(3)
		w.WriteUInt16At(pos, uint16(w.Len()))
	})
}

func (c *QQClient) buildLoginPacket() (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 9,
		List: [][]byte{
			tlv.T18(16, uint32(c.Uin)),
			tlv.T1(uint32(c.Uin), c.Device().IpAddress),
			tlv.T106(uint32(c.Uin), 0, c.version().AppId, c.version().SSOVersion, c.PasswordMd5, true, c.Device().Guid, c.Device().TgtgtKey, 0),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T100(c.version().SSOVersion, c.version().SubAppId, c.version().MainSigMap),
			tlv.T107(0),
			tlv.T142(c.version().ApkId),
			tlv.T144(
				[]byte(c.Device().IMEI),
				c.Device().GenDeviceInfoData(),
				c.Device().OSType,
				c.Device().Version.Release,
				c.Device().SimInfo,
				c.Device().APN,
				false, true, false, tlv.GuidFlag(),
				c.Device().Model,
				c.Device().Guid,
				c.Device().Brand,
				c.Device().TgtgtKey,
			),
			tlv.T145(c.Device().Guid),
			tlv.T147(16, []byte(c.version().SortVersionName), c.version().ApkSign),
			/*
				if (miscBitMap & 0x80) != 0{
					w.Write(tlv.T166(1))
				}
			*/
			tlv.T154(seq),
			tlv.T141(c.Device().SimInfo, c.Device().APN),
			tlv.T8(2052),
			tlv.T511([]string{
				"tenpay.com", "openmobile.qq.com", "docs.qq.com", "connect.qq.com",
				"qzone.qq.com", "vip.qq.com", "gamecenter.qq.com", "qun.qq.com", "game.qq.com",
				"qqweb.qq.com", "office.qq.com", "ti.qq.com", "mail.qq.com", "mma.qq.com",
			}),
			tlv.T187(c.Device().MacAddress),
			tlv.T188(c.Device().AndroidId),
		},
	}
	if len(c.Device().IMSIMd5) != 0 {
		t.Append(tlv.T194(c.Device().IMSIMd5))
	}
	if c.AllowSlider {
		t.Append(tlv.T191(0x82))
	}
	if len(c.Device().WifiBSSID) != 0 && len(c.Device().WifiSSID) != 0 {
		t.Append(tlv.T202(c.Device().WifiBSSID, c.Device().WifiSSID))
	}
	t.Append(
		tlv.T177(c.version().BuildTime, c.version().SdkVersion),
		tlv.T516(),
		tlv.T521(0),
		tlv.T525(tlv.T536([]byte{0x01, 0x00})),
	)
	if wrapper.DandelionEnergy != nil {
		salt := binary.NewWriterF(func(w *binary.Writer) {
			//  util.int64_to_buf(bArr42, 0, (int) uin2);
			//  util.int16_to_buf(bArr42, 4, u.guid.length); // 故意的还是不小心的
			w.Write(binary.NewWriterF(func(w *binary.Writer) { w.WriteUInt64(uint64(c.Uin)) })[:4])
			w.WriteBytesShort(c.Device().Guid)
			w.WriteBytesShort([]byte(c.version().SdkVersion))
			w.WriteUInt32(9) // sub command
			w.WriteUInt32(0) // 被演了
		})
		if t544 := tlv.T544Custom(uint64(c.Uin), "810_9", salt, wrapper.DandelionEnergy); t544 != nil {
			t.Append(t544)
		}
	}
	if c.Device().QImei16 != "" {
		t.Append(tlv.T545([]byte(c.Device().QImei16)))
	} else {
		t.Append(tlv.T545([]byte(c.Device().IMEI)))
	}
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, t)
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildDeviceLockLoginPacket() (uint16, []byte) {
	seq := c.nextSeq()
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, &oicq.TLV{
		Command: 20,
		List: [][]byte{
			tlv.T8(2052),
			tlv.T104(c.sig.T104),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T401(c.sig.G),
		},
	})
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildQRCodeFetchRequestPacket(size, margin, ecLevel uint32) (uint16, []byte) {
	// old := c.version()
	// watch := auth.AndroidWatch.Version()
	// c.transport.Version = watch
	seq := c.nextSeq()
	req := oicq.Message{
		Command:          0x0812,
		EncryptionMethod: oicq.EM_ECDH,
		Body: binary.NewWriterF(func(w *binary.Writer) {
			code2dPacket := buildCode2DRequestPacket(0, 0, 0x31, func(w *binary.Writer) {
				w.WriteUInt16(0)  // const
				w.WriteUInt32(16) // app id
				w.WriteUInt64(0)  // const
				w.WriteByte(8)    // const
				w.WriteBytesShort(EmptyBytes)

				w.WriteUInt16(6)
				w.Write(tlv.T16(c.transport.Version.SSOVersion, 16, c.transport.Version.AppId, c.Device().Guid, []byte(c.transport.Version.ApkId), []byte(c.transport.Version.SortVersionName), c.transport.Version.ApkSign))
				w.Write(tlv.T1B(0, 0, size, margin, 72, ecLevel, 2))
				w.Write(tlv.T1D(c.transport.Version.MiscBitmap))
				w.Write(tlv.T1F(false, c.Device().OSType, []byte("7.1.2"), []byte("China Mobile GSM"), c.Device().APN, 2))
				w.Write(tlv.T33(c.Device().Guid))
				w.Write(tlv.T35(8))
			})
			w.WriteByte(0x0)
			w.WriteUInt16(uint16(len(code2dPacket)) + 4)
			w.WriteUInt32(16)
			w.WriteUInt32(0x72)
			w.WriteHex("000000")
			w.WriteUInt32(uint32(time.Now().Unix()))
			w.Write(code2dPacket)
		}),
	}
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         0,
		CommandName: "wtlogin.trans_emp",
		Body:        c.oicq.Marshal(&req),
	}
	payload := c.transport.PackPacket(&r)
	// c.transport.Version = old
	return seq, payload
}

func (c *QQClient) buildQRCodeResultQueryRequestPacket(sig []byte) (uint16, []byte) {
	// old := c.version()
	// c.transport.Version = auth.AndroidWatch.Version()
	seq := c.nextSeq()
	req := oicq.Message{
		Command:          0x0812,
		EncryptionMethod: oicq.EM_ECDH,
		Body: binary.NewWriterF(func(w *binary.Writer) {
			code2dPacket := buildCode2DRequestPacket(1, 0, 0x12, func(w *binary.Writer) {
				w.WriteUInt16(5)  // const
				w.WriteByte(1)    // const
				w.WriteUInt32(8)  // product type
				w.WriteUInt32(16) // app id
				w.WriteBytesShort(sig)
				w.WriteUInt64(0) // const
				w.WriteByte(8)   // const
				w.WriteBytesShort(EmptyBytes)
				w.WriteUInt16(0) // const
			})
			w.WriteByte(0x0)
			w.WriteUInt16(uint16(len(code2dPacket)) + 4)
			w.WriteUInt32(16)
			w.WriteUInt32(0x72)
			w.WriteHex("000000")
			w.WriteUInt32(uint32(time.Now().Unix()))
			w.Write(code2dPacket)
		}),
	}
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         0,
		CommandName: "wtlogin.trans_emp",
		Body:        c.oicq.Marshal(&req),
	}
	payload := c.transport.PackPacket(&r)
	// c.transport.Version = old
	return seq, payload
}

func (c *QQClient) buildQRCodeLoginPacket(t106, t16a, t318 []byte) (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 9,
		List: [][]byte{
			tlv.T18(16, uint32(c.Uin)),
			tlv.T1(uint32(c.Uin), c.Device().IpAddress),
			tlv.T(0x106, t106),
			// tlv.T106(uint32(c.Uin), 0, c.version.AppId, c.version.SSOVersion, c.PasswordMd5, true, c.device.Guid, c.device.TgtgtKey, 0),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T100(c.version().SSOVersion, c.version().SubAppId, c.version().MainSigMap),
			tlv.T107(0),
			tlv.T142(c.version().ApkId),
			tlv.T144(
				[]byte(c.Device().IMEI),
				c.Device().GenDeviceInfoData(),
				c.Device().OSType,
				c.Device().Version.Release,
				c.Device().SimInfo,
				c.Device().APN,
				false, true, false, tlv.GuidFlag(),
				c.Device().Model,
				c.Device().Guid,
				c.Device().Brand,
				c.Device().TgtgtKey,
			),
			tlv.T145(c.Device().Guid),
			tlv.T147(16, []byte(c.version().SortVersionName), c.version().ApkSign),
			tlv.T(0x16a, t16a),
			tlv.T154(seq),
			tlv.T141(c.Device().SimInfo, c.Device().APN),
			tlv.T8(2052),
			tlv.T511([]string{
				"tenpay.com", "openmobile.qq.com", "docs.qq.com", "connect.qq.com",
				"qzone.qq.com", "vip.qq.com", "gamecenter.qq.com", "qun.qq.com", "game.qq.com",
				"qqweb.qq.com", "office.qq.com", "ti.qq.com", "mail.qq.com", "mma.qq.com",
			}),
			tlv.T187(c.Device().MacAddress),
			tlv.T188(c.Device().AndroidId),
			tlv.T194(c.Device().IMSIMd5),
			tlv.T191(0x00),
			tlv.T202(c.Device().WifiBSSID, c.Device().WifiSSID),
			tlv.T177(c.version().BuildTime, c.version().SdkVersion),
			tlv.T516(),
			tlv.T521(8),
			// tlv.T525(tlv.T536([]byte{0x01, 0x00})),
			tlv.T(0x318, t318),
		},
	}
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, t)
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildCaptchaPacket(result string, sign []byte) (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 2,
		List: [][]byte{
			tlv.T2(result, sign),
			tlv.T8(2052),
			tlv.T104(c.sig.T104),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
		},
	}
	if c.sig.T547 != nil {
		t.Append(tlv.T(0x547, c.sig.T547))
	}
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, t)
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildSMSRequestPacket() (uint16, []byte) {
	seq := c.nextSeq()
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, &oicq.TLV{
		Command: 8,
		List: [][]byte{
			tlv.T8(2052),
			tlv.T104(c.sig.T104),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T174(c.sig.T174),
			tlv.T17A(9),
			tlv.T197(),
		},
	})
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildSMSCodeSubmitPacket(code string) (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 7,
		List: [][]byte{
			tlv.T8(2052),
			tlv.T104(c.sig.T104),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T174(c.sig.T174),
			tlv.T17C(code),
			tlv.T401(c.sig.G),
			tlv.T198(),
		},
	}
	if wrapper.DandelionEnergy != nil {
		if t544 := tlv.T544(uint64(c.Uin), "810_7", 7, c.version().SdkVersion, c.Device().Guid, wrapper.DandelionEnergy); t544 != nil {
			t.Append(t544)
		}
	}
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, t)
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildTicketSubmitPacket(ticket string) (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 2,
		List: [][]byte{
			tlv.T193(ticket),
			tlv.T8(2052),
			tlv.T104(c.sig.T104),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
		},
	}
	if c.sig.T547 != nil {
		t.Append(tlv.T(0x547, c.sig.T547))
	}
	if wrapper.DandelionEnergy != nil {
		if t544 := tlv.T544(uint64(c.Uin), "810_2", 2, c.version().SdkVersion, c.Device().Guid, wrapper.DandelionEnergy); t544 != nil {
			t.Append(t544)
		}
	}
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, t)
	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.login",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildRequestTgtgtNopicsigPacket() (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 15,
		List: [][]byte{
			tlv.T18(16, uint32(c.Uin)),
			tlv.T1(uint32(c.Uin), c.Device().IpAddress),
			tlv.T(0x106, c.sig.EncryptedA1),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T100(c.version().SSOVersion, 2, c.version().MainSigMap),
			tlv.T107(0),
			tlv.T108(c.sig.Ksid),
			tlv.T144(
				c.Device().AndroidId,
				c.Device().GenDeviceInfoData(),
				c.Device().OSType,
				c.Device().Version.Release,
				c.Device().SimInfo,
				c.Device().APN,
				false, true, false, tlv.GuidFlag(),
				c.Device().Model,
				c.Device().Guid,
				c.Device().Brand,
				c.Device().TgtgtKey,
			),
			tlv.T142(c.version().ApkId),
			tlv.T145(c.Device().Guid),
			tlv.T16A(c.sig.SrmToken),
			tlv.T154(seq),
			tlv.T141(c.Device().SimInfo, c.Device().APN),
			tlv.T8(2052),
			tlv.T511([]string{
				"tenpay.com", "openmobile.qq.com", "docs.qq.com", "connect.qq.com",
				"qzone.qq.com", "vip.qq.com", "qun.qq.com", "game.qq.com", "qqweb.qq.com",
				"office.qq.com", "ti.qq.com", "mail.qq.com", "qzone.com", "mma.qq.com",
			}),
			tlv.T147(16, []byte(c.version().SortVersionName), c.version().ApkSign),
			tlv.T177(c.version().BuildTime, c.version().SdkVersion),
			tlv.T400(c.sig.G, c.Uin, c.Device().Guid, c.sig.Dpwd, 1, 16, c.sig.RandSeed),
			tlv.T187(c.Device().MacAddress),
			tlv.T188(c.Device().AndroidId),
			tlv.T194(c.Device().IMSIMd5),
			tlv.T202(c.Device().WifiBSSID, c.Device().WifiSSID),
			tlv.T516(),
			tlv.T521(0),
			tlv.T525(tlv.T536([]byte{0x01, 0x00})),
		},
	}
	if c.Device().QImei16 != "" {
		t.Append(tlv.T545([]byte(c.Device().QImei16)))
	} else {
		t.Append(tlv.T545([]byte(c.Device().IMEI)))
	}
	m := oicq.Message{
		Uin:              uint32(c.Uin),
		Command:          0x810,
		EncryptionMethod: oicq.EM_ST,
		Body:             t.Marshal(),
	}
	req := network.Request{
		Type:        network.RequestTypeSimple,
		EncryptType: network.EncryptTypeEmptyKey,
		Uin:         c.Uin,
		SequenceID:  int32(seq),
		CommandName: "wtlogin.exchange_emp",
		Body:        c.oicq.Marshal(&m),
	}
	return seq, c.transport.PackPacket(&req)
}

func (c *QQClient) buildRequestChangeSigPacket(changeD2 bool) (uint16, []byte) {
	seq := c.nextSeq()
	t := &oicq.TLV{
		Command: 11,
		List: [][]byte{
			tlv.T100(c.version().SSOVersion, 100, c.version().MainSigMap),
			tlv.T10A(c.sig.TGT),
			tlv.T116(c.version().MiscBitmap, c.version().SubSigmap),
			tlv.T108(c.sig.Ksid),
		},
	}
	if !changeD2 {
		t.Command = 10
	}
	var key []byte
	if changeD2 {
		h := md5.Sum(c.sig.D2Key)
		key = h[:]
	} else {
		key = c.sig.TGTKey
	}
	t.Append(
		tlv.T144(
			c.Device().AndroidId,
			c.Device().GenDeviceInfoData(),
			c.Device().OSType,
			c.Device().Version.Release,
			c.Device().SimInfo,
			c.Device().APN,
			false, true, false, tlv.GuidFlag(),
			c.Device().Model,
			c.Device().Guid,
			c.Device().Brand,
			key,
		),
		tlv.T112(c.Uin),
	)
	if changeD2 {
		t.Append(tlv.T143(c.sig.D2))
	} else {
		t.Append(tlv.T145(c.Device().Guid))
	}
	t.Append(
		tlv.T142(c.version().ApkId),
		tlv.T154(seq),
		tlv.T18(16, uint32(c.Uin)),
		tlv.T141(c.Device().SimInfo, c.Device().APN),
		tlv.T8(2052),
		tlv.T147(16, []byte(c.version().SortVersionName), c.version().ApkSign),
		tlv.T177(c.version().BuildTime, c.version().SdkVersion),
		tlv.T187(c.Device().MacAddress),
		tlv.T188(c.Device().AndroidId),
		tlv.T194(c.Device().IMSIMd5),
		tlv.T511([]string{
			"tenpay.com", "openmobile.qq.com", "docs.qq.com", "connect.qq.com",
			"qzone.qq.com", "vip.qq.com", "qun.qq.com", "game.qq.com", "qqweb.qq.com",
			"office.qq.com", "ti.qq.com", "mail.qq.com", "qzone.com", "mma.qq.com",
		}),
		tlv.T202(c.Device().WifiBSSID, c.Device().WifiSSID),
	)
	req := c.buildOicqRequestPacket(c.Uin, 0x0810, t)
	req2 := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeEmptyKey,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "wtlogin.exchange_emp",
		Body:        req,
	}
	return seq, c.transport.PackPacket(&req2)
}

// StatSvc.register
func (c *QQClient) buildClientRegisterPacket() (uint16, []byte) {
	seq := c.nextSeq()
	svc := &jce.SvcReqRegister{
		ConnType:     0,
		Uin:          c.Uin,
		Bid:          1 | 2 | 4,
		Status:       11,
		KickPC:       0,
		KickWeak:     0,
		IOSVersion:   int64(c.Device().Version.SDK),
		NetType:      1,
		RegType:      0,
		Guid:         c.Device().Guid,
		IsSetStatus:  0,
		LocaleId:     2052,
		DevName:      string(c.Device().Model),
		DevType:      string(c.Device().Model),
		OSVer:        string(c.Device().Version.Release),
		OpenPush:     1,
		LargeSeq:     1551,
		OldSSOIp:     0,
		NewSSOIp:     31806887127679168,
		ChannelNo:    "",
		CPID:         0,
		VendorName:   string(c.Device().VendorName),
		VendorOSName: string(c.Device().VendorOSName),
		B769:         []byte{0x0A, 0x04, 0x08, 0x2E, 0x10, 0x00, 0x0A, 0x05, 0x08, 0x9B, 0x02, 0x10, 0x00},
		SetMute:      0,
	}
	b := append([]byte{0x0A}, svc.ToBytes()...)
	b = append(b, 0x0B)
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"SvcReqRegister": b},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		SServantName: "PushService",
		SFuncName:    "SvcReqRegister",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}

	r := network.Request{
		Type:        network.RequestTypeLogin,
		EncryptType: network.EncryptTypeD2Key,
		SequenceID:  int32(seq),
		Uin:         c.Uin,
		CommandName: "StatSvc.register",
		Body:        pkt.ToBytes(),
	}
	return seq, c.transport.PackPacket(&r)
}

func (c *QQClient) buildStatusSetPacket(status, extStatus int32) (uint16, []byte) {
	svc := &jce.SvcReqRegister{
		ConnType:        0,
		Uin:             c.Uin,
		Bid:             7,
		Status:          status,
		KickPC:          0,
		KickWeak:        0,
		Timestamp:       time.Now().Unix(),
		IOSVersion:      int64(c.Device().Version.SDK),
		NetType:         1,
		RegType:         0,
		Guid:            c.Device().Guid,
		IsSetStatus:     1,
		LocaleId:        2052,
		DevName:         string(c.Device().Model),
		DevType:         string(c.Device().Model),
		OSVer:           string(c.Device().Version.Release),
		OpenPush:        1,
		LargeSeq:        1551,
		ExtOnlineStatus: int64(extStatus),
	}
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"SvcReqRegister": packUniRequestData(svc.ToBytes())},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		SServantName: "PushService",
		SFuncName:    "SvcReqRegister",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacket("StatSvc.SetStatusFromClient", pkt.ToBytes())
}

// ConfigPushSvc.PushResp
func (c *QQClient) buildConfPushRespPacket(t int32, pktSeq int64, jceBuf []byte) (uint16, []byte) {
	req := jce.NewJceWriter()
	req.WriteInt32(t, 1)
	req.WriteInt64(pktSeq, 2)
	req.WriteBytes(jceBuf, 3)
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"PushResp": packUniRequestData(req.Bytes())},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		SServantName: "QQService.ConfigPushSvc.MainServant",
		SFuncName:    "PushResp",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacket("ConfigPushSvc.PushResp", pkt.ToBytes())
}

// friendlist.getFriendGroupList
func (c *QQClient) buildFriendGroupListRequestPacket(friendStartIndex, friendListCount, groupStartIndex, groupListCount int16) (uint16, []byte) {
	d50, _ := proto.Marshal(&pb.D50ReqBody{
		Appid:                   1002,
		ReqMusicSwitch:          1,
		ReqMutualmarkAlienation: 1,
		ReqKsingSwitch:          1,
		ReqMutualmarkLbsshare:   1,
	})
	req := &jce.FriendListRequest{
		Reqtype: 3,
		IfReflush: func() byte {
			if friendStartIndex <= 0 {
				return 0
			}
			return 1
		}(),
		Uin:         c.Uin,
		StartIndex:  friendStartIndex,
		FriendCount: friendListCount,
		GroupId:     0,
		IfGetGroupInfo: func() byte {
			if groupListCount <= 0 {
				return 0
			}
			return 1
		}(),
		GroupStartIndex: byte(groupStartIndex),
		GroupCount:      byte(groupListCount),
		IfGetMSFGroup:   0,
		IfShowTermType:  1,
		Version:         27,
		UinList:         nil,
		AppType:         0,
		IfGetDOVId:      0,
		IfGetBothFlag:   0,
		D50:             d50,
		D6B:             EmptyBytes,
		SnsTypeList:     []int64{13580, 13581, 13582},
	}
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"FL": packUniRequestData(req.ToBytes())},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		CPacketType:  0x003,
		IRequestId:   1921334514,
		SServantName: "mqq.IMService.FriendListServiceServantObj",
		SFuncName:    "GetFriendListReq",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacket("friendlist.getFriendGroupList", pkt.ToBytes())
}

// SummaryCard.ReqSummaryCard
func (c *QQClient) buildSummaryCardRequestPacket(target int64) (uint16, []byte) {
	seq := c.nextSeq()
	packBusinessBuf := func(t int32, buf []byte) []byte {
		return binary.NewWriterF(func(w *binary.Writer) {
			comm, _ := proto.Marshal(&profilecard.BusiComm{
				Ver:      proto.Int32(1),
				Seq:      proto.Int32(int32(seq)),
				Fromuin:  proto.Some(c.Uin),
				Touin:    proto.Some(target),
				Service:  proto.Some(t),
				Platform: proto.Int32(2),
				Qqver:    proto.String("8.4.18.4945"),
				Build:    proto.Int32(4945),
			})
			w.WriteByte(40)
			w.WriteUInt32(uint32(len(comm)))
			w.WriteUInt32(uint32(len(buf)))
			w.Write(comm)
			w.Write(buf)
			w.WriteByte(41)
		})
	}
	gate, _ := proto.Marshal(&profilecard.GateVaProfileGateReq{
		UCmd:           proto.Int32(3),
		StPrivilegeReq: &profilecard.GatePrivilegeBaseInfoReq{UReqUin: proto.Some(target)},
		StGiftReq:      &profilecard.GateGetGiftListReq{Uin: proto.Int32(int32(target))},
		StVipCare:      &profilecard.GateGetVipCareReq{Uin: proto.Some(target)},
		OidbFlag: []*profilecard.GateOidbFlagInfo{
			{
				Fieled: proto.Int32(42334),
			},
			{
				Fieled: proto.Int32(42340),
			},
			{
				Fieled: proto.Int32(42344),
			},
			{
				Fieled: proto.Int32(42354),
			},
		},
	})
	/*
		e5b, _ := proto.Marshal(&oidb.DE5BReqBody{
			Uin:                   proto.Uint64(uint64(target)),
			MaxCount:              proto.Uint32(10),
			ReqAchievementContent: proto.Bool(false),
		})
		ec4, _ := proto.Marshal(&oidb.DEC4ReqBody{
			Uin:       proto.Uint64(uint64(target)),
			QuestNum:  proto.Uint64(10),
			FetchType: proto.Uint32(1),
		})
	*/
	req := &jce.SummaryCardReq{
		Uin:              target,
		ComeFrom:         31,
		GetControl:       69181,
		AddFriendSource:  3001,
		SecureSig:        []byte{0x00},
		ReqMedalWallInfo: 0,
		Req0x5ebFieldId:  []int64{27225, 27224, 42122, 42121, 27236, 27238, 42167, 42172, 40324, 42284, 42326, 42325, 42356, 42363, 42361, 42367, 42377, 42425, 42505, 42488},
		ReqServices:      [][]byte{packBusinessBuf(16, gate)},
		ReqNearbyGodInfo: 1,
		ReqExtendCard:    1,
	}
	head := jce.NewJceWriter()
	head.WriteInt32(2, 0)
	buf := &jce.RequestDataVersion3{Map: map[string][]byte{
		"ReqHead":        packUniRequestData(head.Bytes()),
		"ReqSummaryCard": packUniRequestData(req.ToBytes()),
	}}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		SServantName: "SummaryCardServantObj",
		SFuncName:    "ReqSummaryCard",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return seq, c.uniPacketWithSeq(seq, "SummaryCard.ReqSummaryCard", pkt.ToBytes())
}

// friendlist.delFriend
func (c *QQClient) buildFriendDeletePacket(target int64) (uint16, []byte) {
	req := &jce.DelFriendReq{
		Uin:     c.Uin,
		DelUin:  target,
		DelType: 2,
		Version: 1,
	}
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"DF": packUniRequestData(req.ToBytes())},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		IRequestId:   c.nextPacketSeq(),
		SServantName: "mqq.IMService.FriendListServiceServantObj",
		SFuncName:    "DelFriendReq",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacket("friendlist.delFriend", pkt.ToBytes())
}

// friendlist.GetTroopListReqV2
func (c *QQClient) buildGroupListRequestPacket(vecCookie []byte) (uint16, []byte) {
	req := &jce.TroopListRequest{
		Uin:              c.Uin,
		GetMSFMsgFlag:    1,
		Cookies:          vecCookie,
		GroupInfo:        []int64{},
		GroupFlagExt:     1,
		Version:          7,
		CompanyId:        0,
		VersionNum:       1,
		GetLongGroupName: 1,
	}
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"GetTroopListReqV2Simplify": packUniRequestData(req.ToBytes())},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		CPacketType:  0x00,
		IRequestId:   c.nextPacketSeq(),
		SServantName: "mqq.IMService.FriendListServiceServantObj",
		SFuncName:    "GetTroopListReqV2Simplify",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacket("friendlist.GetTroopListReqV2", pkt.ToBytes())
}

// friendlist.GetTroopMemberListReq
func (c *QQClient) buildGroupMemberListRequestPacket(groupUin, groupCode, nextUin int64) (uint16, []byte) {
	req := &jce.TroopMemberListRequest{
		Uin:       c.Uin,
		GroupCode: groupCode,
		NextUin:   nextUin,
		GroupUin:  groupUin,
		Version:   2,
	}
	b := append([]byte{0x0A}, req.ToBytes()...)
	b = append(b, 0x0B)
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"GTML": b},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		IRequestId:   c.nextPacketSeq(),
		SServantName: "mqq.IMService.FriendListServiceServantObj",
		SFuncName:    "GetTroopMemberListReq",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacket("friendlist.GetTroopMemberListReq", pkt.ToBytes())
}

// group_member_card.get_group_member_card_info
func (c *QQClient) buildGroupMemberInfoRequestPacket(groupCode, uin int64) (uint16, []byte) {
	req := &pb.GroupMemberReqBody{
		GroupCode:       groupCode,
		Uin:             uin,
		NewClient:       true,
		ClientType:      1,
		RichCardNameVer: 1,
	}
	payload, _ := proto.Marshal(req)
	return c.uniPacket("group_member_card.get_group_member_card_info", payload)
}

// MessageSvc.PbGetMsg
func (c *QQClient) buildGetMessageRequestPacket(flag msg.SyncFlag, msgTime int64) (uint16, []byte) {
	cook := c.sig.SyncCookie
	if cook == nil {
		cook, _ = proto.Marshal(&msg.SyncCookie{
			Time:   proto.Some(msgTime),
			Ran1:   proto.Int64(758330138),
			Ran2:   proto.Int64(2480149246),
			Const1: proto.Int64(1167238020),
			Const2: proto.Int64(3913056418),
			Const3: proto.Int64(0x1D),
		})
	}
	req := &msg.GetMessageRequest{
		SyncFlag:           proto.Some(flag),
		SyncCookie:         cook,
		LatestRambleNumber: proto.Int32(20),
		OtherRambleNumber:  proto.Int32(3),
		OnlineSyncFlag:     proto.Int32(1),
		ContextFlag:        proto.Int32(1),
		MsgReqType:         proto.Int32(1),
		PubaccountCookie:   EmptyBytes,
		MsgCtrlBuf:         EmptyBytes,
		ServerBuf:          EmptyBytes,
	}
	payload, _ := proto.Marshal(req)
	return c.uniPacket("MessageSvc.PbGetMsg", payload)
}

// MessageSvc.PbDeleteMsg
func (c *QQClient) buildDeleteMessageRequestPacket(msg []*pb.MessageItem) (uint16, []byte) {
	req := &pb.DeleteMessageRequest{Items: msg}
	payload, _ := proto.Marshal(req)
	return c.uniPacket("MessageSvc.PbDeleteMsg", payload)
}

// OnlinePush.RespPush
func (c *QQClient) buildDeleteOnlinePushPacket(uin int64, svrip int32, pushToken []byte, seq uint16, delMsg []jce.PushMessageInfo) []byte {
	req := &jce.SvcRespPushMsg{Uin: uin, Svrip: svrip, PushToken: pushToken, DelInfos: []jce.IJceStruct{}}
	for _, m := range delMsg {
		req.DelInfos = append(req.DelInfos, &jce.DelMsgInfo{
			FromUin:    m.FromUin,
			MsgSeq:     m.MsgSeq,
			MsgCookies: m.MsgCookies,
			MsgTime:    m.MsgTime,
		})
	}
	b := packUniRequestData(req.ToBytes())
	buf := &jce.RequestDataVersion3{
		Map: map[string][]byte{"resp": b},
	}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		IRequestId:   int32(seq),
		SServantName: "OnlinePush",
		SFuncName:    "SvcRespPushMsg",
		SBuffer:      buf.ToBytes(),
		Context:      make(map[string]string),
		Status:       make(map[string]string),
	}
	return c.uniPacketWithSeq(seq, "OnlinePush.RespPush", pkt.ToBytes())
}

// LongConn.OffPicUp
func (c *QQClient) buildOffPicUpPacket(target int64, md5 []byte, size int32) (uint16, []byte) {
	req := &cmd0x352.ReqBody{
		Subcmd: proto.Uint32(1),
		TryupImgReq: []*cmd0x352.D352TryUpImgReq{
			{
				SrcUin:       proto.Uint64(uint64(c.Uin)),
				DstUin:       proto.Uint64(uint64(target)),
				FileMd5:      md5,
				FileSize:     proto.Uint64(uint64(size)),
				FileName:     []byte(fmt.Sprintf("%x.jpg", md5)),
				SrcTerm:      proto.Uint32(5),
				PlatformType: proto.Uint32(9),
				BuType:       proto.Uint32(1),
				PicOriginal:  proto.Bool(true),
				PicType:      proto.Uint32(1000),
				BuildVer:     []byte("8.2.7.4410"),
				FileIndex:    EmptyBytes,
				SrvUpload:    proto.Uint32(1),
				TransferUrl:  EmptyBytes,
			},
		},
	}
	payload, _ := proto.Marshal(req)
	return c.uniPacket("LongConn.OffPicUp", payload)
}

// ProfileService.Pb.ReqSystemMsgNew.Friend
func (c *QQClient) buildSystemMsgNewFriendPacket() (uint16, []byte) {
	req := &structmsg.ReqSystemMsgNew{
		MsgNum:    20,
		Version:   1000,
		Checktype: 2,
		Flag: &structmsg.FlagInfo{
			FrdMsgDiscuss2ManyChat:       1,
			FrdMsgGetBusiCard:            1,
			FrdMsgNeedWaitingMsg:         1,
			FrdMsgUint32NeedAllUnreadMsg: 1,
			GrpMsgMaskInviteAutoJoin:     1,
		},
		FriendMsgTypeFlag: 1,
	}
	payload, _ := proto.Marshal(req)
	return c.uniPacket("ProfileService.Pb.ReqSystemMsgNew.Friend", payload)
}

// friendlist.ModifyGroupCardReq
func (c *QQClient) buildEditGroupTagPacket(groupCode, memberUin int64, newTag string) (uint16, []byte) {
	req := &jce.ModifyGroupCardRequest{
		GroupCode: groupCode,
		UinInfo: []jce.IJceStruct{
			&jce.UinInfo{
				Uin:  memberUin,
				Flag: 31,
				Name: newTag,
			},
		},
	}
	buf := &jce.RequestDataVersion3{Map: map[string][]byte{"MGCREQ": packUniRequestData(req.ToBytes())}}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		IRequestId:   c.nextPacketSeq(),
		SServantName: "mqq.IMService.FriendListServiceServantObj",
		SFuncName:    "ModifyGroupCardReq",
		SBuffer:      buf.ToBytes(),
		Context:      map[string]string{},
		Status:       map[string]string{},
	}
	return c.uniPacket("friendlist.ModifyGroupCardReq", pkt.ToBytes())
}

// OidbSvc.0x8fc_2
func (c *QQClient) buildEditSpecialTitlePacket(groupCode, memberUin int64, newTitle string) (uint16, []byte) {
	body := &oidb.D8FCReqBody{
		GroupCode: proto.Some(groupCode),
		MemLevelInfo: []*oidb.D8FCMemberInfo{
			{
				Uin:                    proto.Some(memberUin),
				UinName:                []byte(newTitle),
				SpecialTitle:           []byte(newTitle),
				SpecialTitleExpireTime: proto.Int32(-1),
			},
		},
	}
	b, _ := proto.Marshal(body)
	payload := c.packOIDBPackage(2300, 2, b)
	return c.uniPacket("OidbSvc.0x8fc_2", payload)
}

// OidbSvc.0x89a_0
func (c *QQClient) buildGroupOperationPacket(body *oidb.D89AReqBody) (uint16, []byte) {
	b, _ := proto.Marshal(body)
	payload := c.packOIDBPackage(2202, 0, b)
	return c.uniPacket("OidbSvc.0x89a_0", payload)
}

// OidbSvc.0x89a_0
func (c *QQClient) buildGroupNameUpdatePacket(groupCode int64, newName string) (uint16, []byte) {
	body := &oidb.D89AReqBody{
		GroupCode: groupCode,
		StGroupInfo: &oidb.D89AGroupinfo{
			IngGroupName: []byte(newName),
		},
	}
	return c.buildGroupOperationPacket(body)
}

// OidbSvc.0x89a_0
func (c *QQClient) buildGroupMuteAllPacket(groupCode int64, mute bool) (uint16, []byte) {
	shutUpTime := int32(0)
	if mute {
		shutUpTime = 268435455
	}
	body := &oidb.D89AReqBody{
		GroupCode: groupCode,
		StGroupInfo: &oidb.D89AGroupinfo{
			ShutupTime: proto.Some(shutUpTime),
		},
	}
	return c.buildGroupOperationPacket(body)
}

// OidbSvc.0x8a0_0
func (c *QQClient) buildGroupKickPacket(groupCode, memberUin int64, kickMsg string, block bool) (uint16, []byte) {
	flagBlock := 0
	if block {
		flagBlock = 1
	}
	body := &oidb.D8A0ReqBody{
		OptUint64GroupCode: groupCode,
		MsgKickList: []*oidb.D8A0KickMemberInfo{
			{
				OptUint32Operate:   5,
				OptUint64MemberUin: memberUin,
				OptUint32Flag:      int32(flagBlock),
			},
		},
		KickMsg: []byte(kickMsg),
	}
	b, _ := proto.Marshal(body)
	payload := c.packOIDBPackage(2208, 0, b)
	return c.uniPacket("OidbSvc.0x8a0_0", payload)
}

// OidbSvc.0x570_8
func (c *QQClient) buildGroupMutePacket(groupCode, memberUin int64, time uint32) (uint16, []byte) {
	b, cl := binary.OpenWriterF(func(w *binary.Writer) {
		w.WriteUInt32(uint32(groupCode))
		w.WriteByte(32)
		w.WriteUInt16(1)
		w.WriteUInt32(uint32(memberUin))
		w.WriteUInt32(time)
	})
	payload := c.packOIDBPackage(1392, 8, b)
	cl()
	return c.uniPacket("OidbSvc.0x570_8", payload)
}

// OidbSvc.0xed3
func (c *QQClient) buildGroupPokePacket(groupCode, target int64) (uint16, []byte) {
	body := &oidb.DED3ReqBody{
		ToUin:     target,
		GroupCode: groupCode,
	}
	b, _ := proto.Marshal(body)
	payload := c.packOIDBPackage(3795, 1, b)
	return c.uniPacket("OidbSvc.0xed3", payload)
}

// OidbSvc.0xed3
func (c *QQClient) buildFriendPokePacket(target int64) (uint16, []byte) {
	body := &oidb.DED3ReqBody{
		ToUin:  target,
		AioUin: target,
	}
	b, _ := proto.Marshal(body)
	payload := c.packOIDBPackage(3795, 1, b)
	return c.uniPacket("OidbSvc.0xed3", payload)
}

// OidbSvc.0x55c_1
func (c *QQClient) buildGroupAdminSetPacket(groupCode, member int64, flag bool) (uint16, []byte) {
	b, cl := binary.OpenWriterF(func(w *binary.Writer) {
		w.WriteUInt32(uint32(groupCode))
		w.WriteUInt32(uint32(member))
		w.WriteBool(flag)
	})
	payload := c.packOIDBPackage(1372, 1, b)
	cl()
	return c.uniPacket("OidbSvc.0x55c_1", payload)
}

// ProfileService.GroupMngReq
func (c *QQClient) buildQuitGroupPacket(groupCode int64) (uint16, []byte) {
	jw := jce.NewJceWriter()
	jw.WriteInt32(2, 0)
	jw.WriteInt64(c.Uin, 1)
	b, cl := binary.OpenWriterF(func(w *binary.Writer) {
		w.WriteUInt32(uint32(c.Uin))
		w.WriteUInt32(uint32(groupCode))
	})
	jw.WriteBytes(b, 2)
	cl()
	buf := &jce.RequestDataVersion3{Map: map[string][]byte{"GroupMngReq": packUniRequestData(jw.Bytes())}}
	pkt := &jce.RequestPacket{
		IVersion:     3,
		IRequestId:   c.nextPacketSeq(),
		SServantName: "KQQ.ProfileService.ProfileServantObj",
		SFuncName:    "GroupMngReq",
		SBuffer:      buf.ToBytes(),
		Context:      map[string]string{},
		Status:       map[string]string{},
	}
	return c.uniPacket("ProfileService.GroupMngReq", pkt.ToBytes())
}

/* this function is unused
// LightAppSvc.mini_app_info.GetAppInfoById
func (c *QQClient) buildAppInfoRequestPacket(id string) (uint16, []byte) {
	seq := c.nextSeq()
	req := &qweb.GetAppInfoByIdReq{
		AppId:           id,
		NeedVersionInfo: 1,
	}
	b, _ := proto.Marshal(req)
	body := &qweb.QWebReq{
		Seq:        proto.Int64(1),
		Qua:        proto.String("V1_AND_SQ_8.4.8_1492_YYB_D"),
		device: proto.String(c.getWebDeviceInfo()),
		BusiBuff:   b,
		TraceId:    proto.String(fmt.Sprintf("%v_%v_%v", c.Uin, time.Now().Format("0102150405"), rand.Int63())),
	}
	payload, _ := proto.Marshal(body)
	packet := packets.BuildUniPacket(c.Uin, seq, "LightAppSvc.mini_app_info.GetAppInfoById", 1, c.SessionId, EmptyBytes, c.sigInfo.d2Key, payload)
	return seq, packet
}
*/

func (c *QQClient) buildWordSegmentationPacket(data []byte) (uint16, []byte) {
	payload := c.packOIDBPackageProto(3449, 1, &oidb.D79ReqBody{
		Uin:     uint64(c.Uin),
		Content: data,
		Qua:     []byte("and_537065262_8.4.5"),
	})
	return c.uniPacket("OidbSvc.0xd79", payload)
}

type ProfileDetailUpdate map[uint16][]byte

func NewProfileDetailUpdate() ProfileDetailUpdate {
	return map[uint16][]byte{}
}

func (p ProfileDetailUpdate) Nick(value string) ProfileDetailUpdate {
	p[20002] = []byte(value)
	return p
}

func (p ProfileDetailUpdate) Email(value string) ProfileDetailUpdate {
	p[20011] = []byte(value)
	return p
}

func (p ProfileDetailUpdate) PersonalNote(value string) ProfileDetailUpdate {
	p[20019] = []byte(value)
	return p
}

func (p ProfileDetailUpdate) Company(value string) ProfileDetailUpdate {
	p[24008] = []byte(value)
	return p
}

func (p ProfileDetailUpdate) College(value string) ProfileDetailUpdate {
	p[20021] = []byte(value)
	return p
}

// OidbSvc.0x4ff_9_IMCore
func (c *QQClient) buildUpdateProfileDetailPacket(profileRecord map[uint16][]byte) (uint16, []byte) {
	b, cl := binary.OpenWriterF(func(w *binary.Writer) {
		w.WriteUInt32(uint32(c.Uin))
		w.WriteByte(0)
		w.WriteUInt16(uint16(len(profileRecord)))
		for tag, value := range profileRecord {
			w.WriteUInt16(tag)
			w.WriteUInt16(uint16(len(value)))
			w.Write(value)
		}
	})
	payload := c.packOIDBPackage(1279, 9, b)
	cl()
	return c.uniPacket("OidbSvc.0x4ff_9_IMCore", payload)
}

// OidbSvc.0x568_22
func (c *QQClient) buildSetGroupAnonymous(groupCode int64, enable bool) (uint16, []byte) {
	var t byte = 0
	if enable {
		t = 1
	}
	b, cl := binary.OpenWriterF(func(w *binary.Writer) {
		w.WriteUInt32(uint32(groupCode))
		w.WriteByte(t)
	})
	payload := c.packOIDBPackage(1384, 22, b)
	cl()
	return c.uniPacket("OidbSvc.0x568_22", payload)
}
