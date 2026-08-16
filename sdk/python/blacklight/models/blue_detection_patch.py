from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.alert_severity import AlertSeverity
from ..models.detection_category import DetectionCategory
from ..models.detection_modifier import DetectionModifier
from ..models.protection import Protection
from ..types import UNSET, Unset
from typing import cast
import datetime


T = TypeVar("T", bound="BlueDetectionPatch")


@_attrs_define
class BlueDetectionPatch:
    """Blue-side only PATCH body for an execution. `version` is the
    optimistic-lock field and is required on every call. Red fields
    are not present — red writes through a separate endpoint with
    its own type.

    """

    version: int
    """ The version the caller read. Mismatch → 409. """
    detection_category: DetectionCategory | Unset = UNSET
    """ Blue-side detection rating, ordinal 0–4. """
    detection_modifiers: list[DetectionModifier] | Unset = UNSET
    """ Qualifiers on the detection category. Multi-select; empty array allowed. """
    protection: Protection | Unset = UNSET
    """ Blue-side prevention rating. """
    detected_at: datetime.datetime | Unset = UNSET
    """ When the first detection fired (UTC). """
    detecting_source: str | Unset = UNSET
    """ Source of detection (e.g. "Splunk", "Sentinel"). """
    detecting_rule_ref: str | Unset = UNSET
    """ Reference to the detection rule that fired. """
    alert_severity: AlertSeverity | Unset = UNSET
    """ Alert severity level. """
    blue_notes: str | Unset = UNSET
    """ Free-form notes from the blue operator. """
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        version = self.version

        detection_category: str | Unset = UNSET
        if not isinstance(self.detection_category, Unset):
            detection_category = self.detection_category.value

        detection_modifiers: list[str] | Unset = UNSET
        if not isinstance(self.detection_modifiers, Unset):
            detection_modifiers = []
            for detection_modifiers_item_data in self.detection_modifiers:
                detection_modifiers_item = detection_modifiers_item_data.value
                detection_modifiers.append(detection_modifiers_item)

        protection: str | Unset = UNSET
        if not isinstance(self.protection, Unset):
            protection = self.protection.value

        detected_at: str | Unset = UNSET
        if not isinstance(self.detected_at, Unset):
            detected_at = self.detected_at.isoformat()

        detecting_source = self.detecting_source

        detecting_rule_ref = self.detecting_rule_ref

        alert_severity: str | Unset = UNSET
        if not isinstance(self.alert_severity, Unset):
            alert_severity = self.alert_severity.value

        blue_notes = self.blue_notes

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "version": version,
            }
        )
        if detection_category is not UNSET:
            field_dict["detectionCategory"] = detection_category
        if detection_modifiers is not UNSET:
            field_dict["detectionModifiers"] = detection_modifiers
        if protection is not UNSET:
            field_dict["protection"] = protection
        if detected_at is not UNSET:
            field_dict["detectedAt"] = detected_at
        if detecting_source is not UNSET:
            field_dict["detectingSource"] = detecting_source
        if detecting_rule_ref is not UNSET:
            field_dict["detectingRuleRef"] = detecting_rule_ref
        if alert_severity is not UNSET:
            field_dict["alertSeverity"] = alert_severity
        if blue_notes is not UNSET:
            field_dict["blueNotes"] = blue_notes

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        version = d.pop("version")

        _detection_category = d.pop("detectionCategory", UNSET)
        detection_category: DetectionCategory | Unset
        if isinstance(_detection_category, Unset):
            detection_category = UNSET
        else:
            detection_category = DetectionCategory(_detection_category)

        _detection_modifiers = d.pop("detectionModifiers", UNSET)
        detection_modifiers: list[DetectionModifier] | Unset = UNSET
        if _detection_modifiers is not UNSET:
            detection_modifiers = []
            for detection_modifiers_item_data in _detection_modifiers:
                detection_modifiers_item = DetectionModifier(detection_modifiers_item_data)

                detection_modifiers.append(detection_modifiers_item)

        _protection = d.pop("protection", UNSET)
        protection: Protection | Unset
        if isinstance(_protection, Unset):
            protection = UNSET
        else:
            protection = Protection(_protection)

        _detected_at = d.pop("detectedAt", UNSET)
        detected_at: datetime.datetime | Unset
        if isinstance(_detected_at, Unset):
            detected_at = UNSET
        else:
            detected_at = datetime.datetime.fromisoformat(_detected_at)

        detecting_source = d.pop("detectingSource", UNSET)

        detecting_rule_ref = d.pop("detectingRuleRef", UNSET)

        _alert_severity = d.pop("alertSeverity", UNSET)
        alert_severity: AlertSeverity | Unset
        if isinstance(_alert_severity, Unset):
            alert_severity = UNSET
        else:
            alert_severity = AlertSeverity(_alert_severity)

        blue_notes = d.pop("blueNotes", UNSET)

        blue_detection_patch = cls(
            version=version,
            detection_category=detection_category,
            detection_modifiers=detection_modifiers,
            protection=protection,
            detected_at=detected_at,
            detecting_source=detecting_source,
            detecting_rule_ref=detecting_rule_ref,
            alert_severity=alert_severity,
            blue_notes=blue_notes,
        )

        blue_detection_patch.additional_properties = d
        return blue_detection_patch

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
