#!/usr/bin/python3

import enum
import ipaddress
import logging
import socket
import typing

import modules.API.Yandex
import modules.web
import yaml


URL_GET_EXTERNAL_IPv4 = "http://ipecho.net/plain"
DST_GET_EXTERNAL_IP = "a.root-servers.net"
CONFIG_PATH = "/etc/update-ip.yml"


logger = logging.getLogger()
logger.setLevel(logging.INFO)
logger.addHandler(logging.StreamHandler())


class DnsRecordType(enum.Enum):
    A = "A"
    AAAA = "AAAA"


def detect_nat_address(url=URL_GET_EXTERNAL_IPv4) -> str:
    return modules.web.get_url_body(url)


def detect_local_address(addr_type=socket.AF_INET6, dst_addr=DST_GET_EXTERNAL_IP) -> str:
    s = socket.socket(addr_type, socket.SOCK_DGRAM)
    s.connect((dst_addr, 80))

    src_ip = s.getsockname()[0]
    if not src_ip:
        raise ValueError(f"cannot detect AF{socket.AF_INET6} address")

    return src_ip


def main():
    config: typing.Dict[str, typing.Any]
    with open(CONFIG_PATH, "r", encoding="utf-8") as fh:
        config = yaml.load(fh, Loader=yaml.BaseLoader)

    dns_api = modules.API.Yandex.API360(config["organization"], config["domain"], config["token"])
    domains_desc = dns_api.list_domain()
    if "records" not in domains_desc:
        raise ValueError("cannot get domain description")

    detected_addresses: typing.Set(ipaddress._BaseAddress) = set()
    for func_detect_address in (detect_nat_address, detect_local_address):
        try:
            _ip_addr = func_detect_address()
            detected_addresses.add(ipaddress.ip_address(_ip_addr))
        except Exception as err:
            logger.warning("func detect source address exception: %s", err)
            continue

    for addr in detected_addresses:
        _record_exists_flag: bool = False
        record_id: typing.Optional[int] = None

        ip_addr: str = addr.compressed
        record_type: DnsRecordType = DnsRecordType.A.value
        if isinstance(addr, ipaddress.IPv6Address):
            record_type = DnsRecordType.AAAA.value

        for r_desc in domains_desc["records"]:
            if r_desc["type"] == record_type and r_desc["name"] == config["subdomain"]:
                if r_desc["address"] == ip_addr:
                    _record_exists_flag = True
                    break

                record_id = r_desc["recordId"]
                break
        if _record_exists_flag:
            continue

        if record_id is not None:
            message = dns_api.edit_domain(
                **{
                    "record_id": record_id,
                    "address": ip_addr,
                    "name": config["subdomain"],
                    "record_type": record_type,
                    "ttl": config["ttl"],
                }
            )
        else:
            message = dns_api.add_domain(
                **{
                    "address": ip_addr,
                    "name": config["subdomain"],
                    "record_type": record_type,
                    "ttl": config["ttl"],
                }
            )

        logger.debug("message: %s", message)


if __name__ == "__main__":
    main()
